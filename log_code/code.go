package log_code

const (
	WarnHttpServerShutdown = 602
	ErrorHttpServerListen  = 603

	WarnProxyGrpcHandler                = 604 //metadata: {"typeData":"", "method":""}
	WarnConvertErrorDataMarshalResponse = 605

	ErrorClientHttp = 611

	ErrorWebsocketProxy = 618
)
