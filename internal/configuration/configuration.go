package configuration

type Redis struct {
	Addrs     []string
	IsCluster bool
}

type GRPC struct {
	Addr string
}

type NATS struct {
	Addr string

}

type Config struct {
	GRPC  GRPC
	Redis *Redis
	Nats  *NATS
}
