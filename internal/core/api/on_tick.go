package api

func (a *Application) OnTick() {
	guilds := a.botSrv.GetGuilds()
	for _, guild := range guilds {
		go a.UpdateGuildSummaryAndLogError(guild)
	}
}
