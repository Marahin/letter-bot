package api

func (a *Application) OnTick() {
	guilds := a.botSrv.GetGuilds()
	for _, guild := range guilds {
		go a.UpdateGuildSummaryAndLogError(guild)
	}
}

//func (a *Application) OnTick(bot ports.BotPort) {
//	guilds := bot.GetGuilds()
//	for _, guild := range guilds {
//		go a.UpdateGuildSummaryAndLogError(bot, guild)
//		go a.SendPeriodicMessageUnlessRedundant(bot, guild)
//	}
//}
//
//// SendPeriodicMessage sends a message to the command channel, linking to the open source repository and buy me a coffee page.
//// Happens only if last message is not the same, every four ticks.
//func (a *Application) SendPeriodicMessageUnlessRedundant(bot ports.BotPort, guild *discord.Guild) {
//	if a.ticks.Load()%4 != 0 {
//		return
//	}
//	a.ticks.Add(1)
//
//	log := a.log.WithFields(logrus.Fields{"guild.ID": guild.ID, "guild.Name": guild.Name, "name": "SendPeriodicMessage"})
//
//	commandChannel, err := bot.FindChannel(guild, discord.CommandChannel)
//	if err != nil {
//		log.Errorf("could not find command channel: %s", err)
//
//		return
//	}
//
//	msgs, err := bot.ChannelMessages(guild, commandChannel, 1)
//	if err != nil {
//		log.Errorf("could not get channel messages: %s", err)
//
//		return
//	}
//
//	if len(msgs) != 0 {
//		lastMessageContent := msgs[0].Content
//		if lastMessageContent == commonStrings.PeriodicMessageContent {
//			log.Info("skipping periodic message as it is redundant")
//			return
//		}
//	}
//
//	err = bot.SendChannelMessage(guild, commandChannel, commonStrings.PeriodicMessageContent)
//	if err != nil {
//		log.Errorf("could not send periodic message: %s", err)
//
//		return
//	}
//}
