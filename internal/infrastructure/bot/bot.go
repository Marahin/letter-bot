package bot

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/kelseyhightower/envconfig"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/servusdei2018/shards/v2"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/ports"
)

type cfg struct {
	Token           string
	CharactersLimit int `default:"5000"`
}

type Bot struct {
	summarySrv         ports.SummaryService
	reservationRepo    ports.ReservationRepository
	onlineCheckService ports.OnlineCheckService
	eventHandler       ports.APIPort
	metrics            ports.MetricsPort
	mgr                *shards.Manager
	log                *zap.SugaredLogger
	quit               chan struct{}
	formatter          *formatter.DiscordFormatter
	channelLocks       cmap.ConcurrentMap[string, *sync.RWMutex]
	started            atomic.Bool
	stopped            atomic.Bool
}

var (
	Config cfg
)

func init() {
	envconfig.MustProcess("bot", &Config)
}

func NewManager(summarySrv ports.SummaryService, reservationRepo ports.ReservationRepository, checkOnlineSrv ports.OnlineCheckService) *Bot {
	mgr, err := shards.New("Bot " + Config.Token)
	if err != nil {
		panic(fmt.Errorf("could not create shards manager, %w", err))
	}

	mgr.Intent = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates
	bot := &Bot{
		mgr:                mgr,
		quit:               make(chan struct{}),
		channelLocks:       cmap.New[*sync.RWMutex](),
		summarySrv:         summarySrv,
		reservationRepo:    reservationRepo,
		onlineCheckService: checkOnlineSrv,
	}

	bot.mgr.AddHandler(bot.GuildCreate)
	bot.mgr.AddHandler(bot.Ready)
	bot.mgr.AddHandler(bot.InteractionCreate)

	return bot
}

func (b *Bot) WithHttpClient(client *http.Client) {
}

func (b *Bot) WithFormatter(formatter *formatter.DiscordFormatter) *Bot {
	b.formatter = formatter

	return b
}

// WithMetrics sets metrics collector implementation.
func (b *Bot) WithMetrics(m ports.MetricsPort) *Bot {
	b.metrics = m
	return b
}

// WithEVentHandler sets bot's event handler to the provided port
func (b *Bot) WithEventHandler(port ports.APIPort) ports.BotPort {
	b.eventHandler = port
	return b
}

func (b *Bot) WithLogger(log *zap.SugaredLogger) *Bot {
	b.log = log.With("layer", "infrastructure", "name", "bot")

	return b
}

func (b *Bot) Run() error {
	b.log.Info("Starting bot...")
	err := b.mgr.Start()
	if err != nil {
		return err
	}
	b.started.Store(true)

	// Wait here until CTRL-C or other term signal is received.
	b.log.Info("bot is now running")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Manager.
	b.log.Warn("stopping shard manager...")
	err = b.mgr.Shutdown()
	if err != nil {
		return err
	}
	b.stopped.Store(true)

	b.log.Info("shard manager stopped. Bot is shut down.")
	return nil
}

func (b *Bot) Shutdown() error {
	close(b.quit) // Tell other goroutines, such as ticker, to shut down
	return b.mgr.Shutdown()
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
