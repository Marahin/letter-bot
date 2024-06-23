package auth_rest

import (
	"encoding/gob"
	"image"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/mitchellh/mapstructure"

	"spot-assistant/internal/common/collections"
	guild2 "spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/ports"
)

type GuildWithRole struct {
	Guild *guild2.Guild `json:"guild"`
	Roles []*role.Role  `json:"roles"`
}

type AuthenticatedUser struct {
	ProviderID      string           `json:"discord_id"`
	Avatar          image.Image      `json:"avatar"`
	Username        string           `json:"username"`
	GuildsWithRoles []*GuildWithRole `json:"guilds_with_roles"`
}

type RestAuth struct {
	Store sessions.Store

	guildRepo ports.GuildRepository
}

func init() {
	gob.Register(discordgo.User{})
	gob.Register(image.NRGBA{})

	// For some reason GuildWithRole is hoisted to `main` scope, probably by the compiler / CompileDaemon
	// gob.Register(GuildWithRole{})
	gob.RegisterName("[]*main.GuildWithRole", []*GuildWithRole{})
}

func NewRestAuth(guildRepo ports.GuildRepository) RestAuth {
	goth.UseProviders()

	store := sessions.NewCookieStore([]byte("secret"))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   false,
	}
	gothic.Store = store

	return RestAuth{
		guildRepo: guildRepo,
		Store:     store,
	}
}

func (auth RestAuth) ProviderHandler(c echo.Context) error {
	provider := c.Param("provider")
	if provider == "" {
		return c.String(http.StatusBadRequest, "Provider not specified")
	}

	q := c.Request().URL.Query()
	q.Add("provider", c.Param("provider"))
	c.Request().URL.RawQuery = q.Encode()

	req := c.Request()
	res := c.Response().Writer
	if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
		return c.JSON(http.StatusOK, gothUser)
	}
	gothic.BeginAuthHandler(res, req)
	return nil

}

func (auth RestAuth) ProviderCallbackHandler(c echo.Context) error {
	req := c.Request()
	res := c.Response().Writer
	c.Logger().Info("CompleteUserAuth")
	userSession, err := gothic.CompleteUserAuth(res, req)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	sess, err := discordgo.New("Bearer " + userSession.AccessToken)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	self, err := sess.User("@me")
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	cachedGuilds, err := auth.guildRepo.SelectGuilds(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	// Download all user guilds
	var userGuilds []*discordgo.UserGuild
	afterID := ""
	for {
		guilds, err := sess.UserGuilds(100, "", afterID, false)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		// Only download guilds that letter is present in
		userGuilds = append(userGuilds, collections.PoorMansFilter(guilds, func(guild *discordgo.UserGuild) bool {
			_, ind := collections.PoorMansFind(cachedGuilds, func(cachedGuild *guild2.Guild) bool {
				return cachedGuild.ID == guild.ID
			})

			return ind != -1
		})...)

		if (len(guilds)) == 100 {
			afterID = guilds[99].ID
		} else {
			break
		}
	}

	avatar, err := sess.UserAvatarDecode(self)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	var user = &AuthenticatedUser{
		ProviderID:      self.ID,
		GuildsWithRoles: make([]*GuildWithRole, 0),
		Username:        self.Username,
		Avatar:          avatar,
	}

	c.Logger().Warn("iterating over user guilds")
	for _, guild := range userGuilds {
		guildMembership, err := sess.UserGuildMember(guild.ID)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		guildDiscordRoles, err := sess.GuildRoles(guild.ID)
		if err != nil {
			break
		}
		guildRoles := bot.MapRoles(guildDiscordRoles)

		userRoles := collections.PoorMansFilter(guildRoles, func(role *role.Role) bool {
			_, index := collections.PoorMansFind(guildMembership.Roles, func(roleID string) bool {
				return roleID == role.ID
			})

			return index != -1
		})

		user.GuildsWithRoles = append(user.GuildsWithRoles, &GuildWithRole{
			Guild: bot.MapUserGuild(guild),
			Roles: userRoles,
		})
	}

	c.Logger().Warn("mapping user")
	rawUserData := &map[string]interface{}{}
	if err := mapstructure.Decode(user, rawUserData); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	userSession.RawData = *rawUserData

	cookie, err := auth.Store.Get(c.Request(), "session")
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	c.Logger().Warn("writing user session")

	cookie.Values["user"] = userSession
	err = cookie.Save(c.Request(), c.Response())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, user)

}
