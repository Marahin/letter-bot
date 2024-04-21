package api

func (a *Application) OnReady() {
	a.log.Info("OnReady")

	a.botSrv.StartTicking()
}
