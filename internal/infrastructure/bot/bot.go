package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
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

// InteractionGateway is the part of the Discord client that answers
// interactions. shardGateway implements it.
type InteractionGateway interface {
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error
	InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	// Guild asks the REST API.
	Guild(guildID string, options ...discordgo.RequestOption) (*discordgo.Guild, error)
	// StateGuild reads the guild from the gateway cache of its shard.
	StateGuild(guildID string) (*discordgo.Guild, error)
	// StateChannels returns a copy of the guild's channels from the gateway cache of its shard.
	StateChannels(guildID string) ([]*discordgo.Channel, error)
	// GuildMember asks the REST API, e.g. for the member behind a click in a DM.
	GuildMember(guildID, userID string, options ...discordgo.RequestOption) (*discordgo.Member, error)
}

// SummaryChannelWriter is the part of the Discord REST API that maintains the
// summaries and posts the announcements. *discordgo.Session implements it. The bot
// uses one session for all of it, so discordgo tracks every rate limit bucket in
// one place.
type SummaryChannelWriter interface {
	GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error)
	GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelDelete(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string, options ...discordgo.RequestOption) ([]*discordgo.Message, error)
	ChannelMessagesBulkDelete(channelID string, messages []string, options ...discordgo.RequestOption) error
	ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) error
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelFileSend(channelID, name string, r io.Reader, options ...discordgo.RequestOption) (*discordgo.Message, error)
	UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
}

// BookingFulfiller is told about every new booking, the /book command's too.
type BookingFulfiller interface {
	Fulfil(ctx context.Context, guildID, userID string, spotID int64) error
}

type shardGateway struct {
	*discordgo.Session

	mgr *shards.Manager
}

func (g shardGateway) StateGuild(guildID string) (*discordgo.Guild, error) {
	state, err := g.state(guildID)
	if err != nil {
		return nil, err
	}
	return state.Guild(guildID)
}

func (g shardGateway) StateChannels(guildID string) ([]*discordgo.Channel, error) {
	state, err := g.state(guildID)
	if err != nil {
		return nil, err
	}
	gld, err := state.Guild(guildID)
	if err != nil {
		return nil, err
	}
	// State.Guild returns the cached guild itself, which the gateway events change.
	state.RLock()
	defer state.RUnlock()
	return slices.Clone(gld.Channels), nil
}

func (g shardGateway) state(guildID string) (*discordgo.State, error) {
	id, err := strconv.ParseInt(guildID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse guild id: %w", err)
	}
	session := g.mgr.SessionForGuild(id)
	if session == nil || session.State == nil {
		return nil, discordgo.ErrStateNotFound
	}
	return session.State, nil
}

type Bot struct {
	summarySrv         ports.SummaryService
	reservationRepo    ports.ReservationRepository
	spots              ports.SpotRepository
	onlineCheckService ports.OnlineCheckService
	guildConfigs       ports.GuildConfigRepository
	layouts            ports.SummaryLayoutRepository
	passHooks          []SummaryPassHook
	now                func() time.Time
	syncer             *GuildSyncer
	summaries          *summaryWorker
	legacyHashes       cmap.ConcurrentMap[string, string]
	pacer              *pacer
	fulfiller          BookingFulfiller
	queue              ports.SpotQueueService
	eventHandler       ports.APIPort
	forms              ports.ReservationFormService
	metrics            ports.MetricsPort
	mgr                *shards.Manager
	gateway            InteractionGateway
	writer             SummaryChannelWriter
	webBaseURL         string
	log                *zap.SugaredLogger
	quit               chan struct{}
	formatter          *formatter.DiscordFormatter
	channelLocks       cmap.ConcurrentMap[string, *sync.RWMutex]
	ticking            sync.Once
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
		gateway:            shardGateway{Session: mgr.Gateway, mgr: mgr},
		writer:             mgr.Gateway,
		webBaseURL:         webBaseURL,
		quit:               make(chan struct{}),
		channelLocks:       cmap.New[*sync.RWMutex](),
		legacyHashes:       cmap.New[string](),
		pacer:              newPacer(),
		now:                time.Now,
		summarySrv:         summarySrv,
		reservationRepo:    reservationRepo,
		onlineCheckService: checkOnlineSrv,
		log:                zap.NewNop().Sugar(),
	}
	bot.summaries = newSummaryWorker(bot.reconcileSummaries, summaryRefreshDelay)

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

func (b *Bot) WithReservationForms(forms ports.ReservationFormService) *Bot {
	b.forms = forms
	return b
}

// WithSpots sets the spot store, for the names in the booking announcements.
func (b *Bot) WithSpots(spots ports.SpotRepository) *Bot {
	b.spots = spots
	return b
}

// WithSummaryLayouts turns on the region summaries: without the store every
// guild keeps the single summary.
func (b *Bot) WithSummaryLayouts(layouts ports.SummaryLayoutRepository) *Bot {
	b.layouts = layouts
	return b
}

// WithSummaryPassHook adds a step to the start of each summary pass.
func (b *Bot) WithSummaryPassHook(h SummaryPassHook) *Bot {
	b.passHooks = append(b.passHooks, h)
	return b
}

func (b *Bot) clock() time.Time {
	if b.now == nil {
		return time.Now()
	}
	return b.now()
}

// WithBookingFulfiller sets what is told about every new booking. It is optional.
func (b *Bot) WithBookingFulfiller(f BookingFulfiller) *Bot {
	b.fulfiller = f
	return b
}

// WithSpotQueue turns on the respawn queue: the /queue commands, the Queue
// button, the removal of a member's entry when they book the respawn, and the
// offers, which each summary pass sends.
func (b *Bot) WithSpotQueue(q ports.SpotQueueService) *Bot {
	b.queue = q
	b.fulfiller = q
	b.passHooks = append(b.passHooks, b.sendQueueOffers)
	return b
}

func (b *Bot) WithLogger(log *zap.SugaredLogger) *Bot {
	b.log = log.With("layer", "infrastructure", "name", "bot")
	b.summaries.log = log.With("layer", "infrastructure", "name", "summaryWorker")
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

// RunSummaryWorker updates the summaries until ctx ends. It is the only writer of the summaries.
func (b *Bot) RunSummaryWorker(ctx context.Context) {
	b.summaries.Run(ctx)
}

// IsRunning indicates whether the bot started successfully and hasn't been stopped yet.
func (b *Bot) IsRunning() bool {
	return b.started.Load() && !b.stopped.Load()
}

func (b *Bot) interactionRespond(i *discordgo.InteractionCreate, responseData *discordgo.InteractionResponseData, responseType discordgo.InteractionResponseType) error {
	return b.gateway.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: responseType,
		Data: responseData,
	})
}
