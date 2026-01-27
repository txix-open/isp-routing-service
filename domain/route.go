package domain

type RouteKey struct {
	Method        string
	CleanPath     string
	WithApiPrefix bool
}

type RouteConfig struct {
	Transport string
	Addresses []string
}
