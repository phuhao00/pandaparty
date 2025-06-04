package nsqx

import (
	"fmt" // Added
	"log" // Added

	"github.com/nsqio/go-nsq"
	"github.com/phuhao00/pandaparty/config" // Added
)

type Producer struct {
	producer *nsq.Producer
}

func (p *Producer) GetReal() *nsq.Producer {
	return p.producer
}

func NewProducer(cfg config.NSQConfig) (*Producer, error) {
	nsqCfg := nsq.NewConfig() // Can be customized further from cfg if needed in the future

	if len(cfg.NSQDAddresses) > 0 {
		for _, addr := range cfg.NSQDAddresses {
			p, err := nsq.NewProducer(addr, nsqCfg)
			if err == nil {
				log.Printf("NSQ Producer connected to %s from list", addr)
				return &Producer{producer: p}, nil
			}
			log.Printf("NSQ Producer failed to connect to %s from list: %v", addr, err)
		}
		log.Printf("Failed to connect to any NSQD in NSQDAddresses")
		return nil, fmt.Errorf("failed to connect to any NSQD in NSQDAddresses")
	} else if cfg.NSQDAddr != "" {
		p, err := nsq.NewProducer(cfg.NSQDAddr, nsqCfg)
		if err != nil {
			log.Printf("NSQ Producer failed to connect to single NSQDAddr %s: %v", cfg.NSQDAddr, err)
			return nil, err
		}
		log.Printf("NSQ Producer connected to single NSQDAddr %s", cfg.NSQDAddr)
		return &Producer{producer: p}, nil
	} else {
		return nil, fmt.Errorf("no NSQD addresses provided in config (neither nsqd_addr nor nsqd_addresses)")
	}
}

func (p *Producer) Publish(topic string, body []byte) error {
	return p.producer.Publish(topic, body)
}

func (p *Producer) Stop() {
	p.producer.Stop()
}

type Consumer struct {
	consumer *nsq.Consumer
}

func NewConsumer(cfg config.NSQConfig, topic, channel string, handler nsq.Handler) (*Consumer, error) {
	nsqCfg := nsq.NewConfig() // Can be customized further from cfg if needed in the future
	// Note: The topic and channel from cfg.Topic and cfg.Channel might be used as defaults
	// if the parameters `topic` and `channel` are empty, or this function could
	// enforce that `topic` and `channel` parameters must be non-empty.
	// For now, strictly using the passed parameters `topic` and `channel`.
	c, err := nsq.NewConsumer(topic, channel, nsqCfg)
	if err != nil {
		return nil, err
	}
	c.AddHandler(handler)
	return &Consumer{consumer: c}, nil
}

func (c *Consumer) ConnectToNSQD(nsqdAddr string) error {
	log.Printf("NSQ Consumer connecting to single NSQD: %s", nsqdAddr)
	return c.consumer.ConnectToNSQD(nsqdAddr)
}

func (c *Consumer) ConnectToNSQLookupds(lookupdHTTPAddresses []string) error {
	if len(lookupdHTTPAddresses) == 0 {
		return fmt.Errorf("no nsqlookupd http addresses provided to ConnectToNSQLookupds")
	}
	log.Printf("NSQ Consumer connecting to NSQLookupds: %v", lookupdHTTPAddresses)
	return c.consumer.ConnectToNSQLookupds(lookupdHTTPAddresses)
}

func (c *Consumer) Stop() {
	c.consumer.Stop()
}
