package bot

import (
	"os"
	"os/signal"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/kelseyhightower/envconfig"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/servusdei2018/shards/v2"
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/ports"
)

type cfg struct {
	Token           string
	CharactersLimit int `default:"5000"`
}

type Bot struct {
	eventHandler ports.APIPort
	mgr          *shards.Manager
	log          *logrus.Entry
	quit         chan struct{}
	formatter    *formatter.DiscordFormatter
	channelLocks cmap.ConcurrentMap[string, *sync.RWMutex]
}

var (
	Config cfg
)

func init() {
	envconfig.MustProcess("bot", &Config)
}

func NewManager() *Bot {
	// Create a new shard manager using the provided bot token.
	mgr, err := shards.New("Bot " + Config.Token)
	if err != nil {
		logrus.Panic("could not create shards manager,", err)
	}

	mgr.Intent = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	bot := &Bot{
		mgr:          mgr,
		quit:         make(chan struct{}),
		channelLocks: cmap.New[*sync.RWMutex](),
		log:          logrus.WithFields(logrus.Fields{"type": "infra", "name": "bot"}),
	}

	bot.mgr.AddHandler(bot.GuildCreate)
	bot.mgr.AddHandler(bot.Ready)
	bot.mgr.AddHandler(bot.InteractionCreate)

	return bot
}

func (b *Bot) WithFormatter(formatter *formatter.DiscordFormatter) *Bot {
	b.formatter = formatter

	return b
}

// WithEVentHandler sets bot's event handler to the provided port
func (b *Bot) WithEventHandler(port ports.APIPort) ports.BotPort {
	b.eventHandler = port

	return b
}

func (b *Bot) Run() error {
	err := b.mgr.Start()
	if err != nil {
		return err
	}

	// Wait here until CTRL-C or other term signal is received.
	b.log.Info("bot is now running")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Manager.
	b.log.Warning("stopping shard manager...")
	err = b.mgr.Shutdown()
	if err != nil {
		return err
	}

	b.log.Info("shard manager stopped. Bot is shut down.")
	return nil
}

func (b *Bot) Shutdown() error {
	close(b.quit) // Tell other goroutines, such as ticker, to shut down
	return b.mgr.Shutdown()
}

func (b *Bot) interactionRespond(i *discordgo.InteractionCreate, responseData *discordgo.InteractionResponseData, responseType discordgo.InteractionResponseType) error {
	return b.mgr.Gateway.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: responseType,
		Data: responseData,
	})
}
