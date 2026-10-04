package bot

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/kelseyhightower/envconfig"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/servusdei2018/shards/v2"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/ports"
)

const (
	summaryRefreshDelay  = 3 * time.Second
	shardConnectAttempts = 5
	shardBackoffBase     = 2 * time.Second
	shardBackoffCap      = 30 * time.Second
)

// newShardManager is a seam so tests can stub shards.New.
var newShardManager = shards.New

func shardBackoff(attempt int) time.Duration {
	delay := shardBackoffBase << attempt
	if delay > shardBackoffCap || delay <= 0 {
		return shardBackoffCap
	}
	return delay
}

// connectManager retries create with backoff, sleeping between attempts only.
// It returns the last error after exhausting attempts.
func connectManager(create func(string) (*shards.Manager, error), token string, attempts int, delay func(attempt int) time.Duration) (*shards.Manager, error) {
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		var mgr *shards.Manager
		mgr, err = create(token)
		if err == nil {
			return mgr, nil
		}
		if attempt < attempts-1 {
			time.Sleep(delay(attempt))
		}
	}
	return nil, err
}

// Config is read from BOT_*. A field with an envconfig tag falls back to the
// unprefixed name, so WEB_BASE_URL and METRICS_ADDR also work.
type Config struct {
	Token string
	// WebBaseURL is optional.
	WebBaseURL  string `envconfig:"WEB_BASE_URL"`
	MetricsAddr string `envconfig:"METRICS_ADDR" default:":2112"`
}

// LoadConfig reads BOT_* from the environment.
func LoadConfig() (Config, error) {
	var cfg Config
	if err := envconfig.Process("bot", &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Token == "" {
		return Config{}, errors.New("BOT_TOKEN must be set")
	}
	return cfg, nil
}

// ConnectShards creates the shard manager. It asks Discord for the gateway, with retries.
func ConnectShards(token string) (*shards.Manager, error) {
	return connectShardsWith(token, shardBackoff)
}

func connectShardsWith(token string, delay func(attempt int) time.Duration) (*shards.Manager, error) {
	mgr, err := connectManager(newShardManager, "Bot "+token, shardConnectAttempts, delay)
	if err != nil {
		return nil, fmt.Errorf("could not create shards manager after %d attempts: %w", shardConnectAttempts, err)
	}
	return mgr, nil
}

type Bot struct {
	summarySrv         ports.SummaryService
	reservationRepo    ports.ReservationRepository
	onlineCheckService ports.OnlineCheckService
	guildConfigs       ports.GuildConfigRepository
	syncer             *GuildSyncer
	summaryRefresh     *debouncer
	eventHandler       ports.APIPort
	metrics            ports.MetricsPort
	mgr                *shards.Manager
	webBaseURL         string
	log                *zap.SugaredLogger
	quit               chan struct{}
	formatter          *formatter.DiscordFormatter
	channelLocks       cmap.ConcurrentMap[string, *sync.RWMutex]
	started            atomic.Bool
	stopped            atomic.Bool
	shutdown           sync.Once
	shutdownErr        error
}

// NewManager builds the bot over mgr. webBaseURL may be empty.
func NewManager(mgr *shards.Manager, webBaseURL string, summarySrv ports.SummaryService, reservationRepo ports.ReservationRepository, checkOnlineSrv ports.OnlineCheckService) *Bot {
	mgr.Intent = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates
	bot := &Bot{
		mgr:                mgr,
		webBaseURL:         webBaseURL,
		quit:               make(chan struct{}),
		channelLocks:       cmap.New[*sync.RWMutex](),
		summarySrv:         summarySrv,
		reservationRepo:    reservationRepo,
		onlineCheckService: checkOnlineSrv,
		summaryRefresh:     newDebouncer(summaryRefreshDelay),
	}

	bot.mgr.AddHandler(bot.GuildCreate)
	bot.mgr.AddHandler(bot.GuildUpdate)
	bot.mgr.AddHandler(bot.GuildDelete)
	bot.mgr.AddHandler(bot.ChannelCreate)
	bot.mgr.AddHandler(bot.ChannelUpdate)
	bot.mgr.AddHandler(bot.ChannelDelete)
	bot.mgr.AddHandler(bot.GuildRoleCreate)
	bot.mgr.AddHandler(bot.GuildRoleUpdate)
	bot.mgr.AddHandler(bot.GuildRoleDelete)
	bot.mgr.AddHandler(bot.Ready)
	bot.mgr.AddHandler(bot.InteractionCreate)

	return bot
}

func (b *Bot) WithFormatter(f *formatter.DiscordFormatter) *Bot {
	b.formatter = f

	return b
}

// WithGuildRepositories sets the guild configuration store and the channel and role sync.
func (b *Bot) WithGuildRepositories(configs ports.GuildConfigRepository, channels ports.GuildChannelRepository, roles ports.GuildRoleRepository) *Bot {
	b.guildConfigs = configs
	b.syncer = NewGuildSyncer(b.mgr.Gateway, channels, roles, configs)
	if b.log != nil {
		b.syncer.WithLogger(b.log)
	}
	return b
}

// WithMetrics sets metrics collector implementation.
func (b *Bot) WithMetrics(m ports.MetricsPort) *Bot {
	b.metrics = m
	return b
}

// WithEventHandler sets the bot's event handler.
func (b *Bot) WithEventHandler(port ports.APIPort) *Bot {
	b.eventHandler = port
	return b
}

func (b *Bot) WithLogger(log *zap.SugaredLogger) *Bot {
	b.log = log.With("layer", "infrastructure", "name", "bot")
	if b.syncer != nil {
		b.syncer.WithLogger(log)
	}

	return b
}

// Start opens the gateway connections.
func (b *Bot) Start() error {
	b.log.Info("Starting bot...")
	if err := b.mgr.Start(); err != nil {
		return err
	}
	b.started.Store(true)
	b.log.Info("bot is now running")
	return nil
}

// Shutdown stops the ticker and closes the gateway connections. Only the first call has an effect.
func (b *Bot) Shutdown() error {
	b.shutdown.Do(func() {
		close(b.quit)
		b.shutdownErr = b.mgr.Shutdown()
		b.stopped.Store(true)
	})
	return b.shutdownErr
}

// IsRunning indicates whether the bot started successfully and hasn't been stopped yet.
func (b *Bot) IsRunning() bool {
	return b.started.Load() && !b.stopped.Load()
}

func (b *Bot) interactionRespond(i *discordgo.InteractionCreate, responseData *discordgo.InteractionResponseData, responseType discordgo.InteractionResponseType) error {
	return b.mgr.Gateway.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: responseType,
		Data: responseData,
	})
}
