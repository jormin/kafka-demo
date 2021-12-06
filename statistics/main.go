package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Shopify/sarama"
	"github.com/rs/zerolog"
)

func main() {
	logger := GetLogger()
	servers := []string{
		"kafka-0.kafka-headless.devops.svc.cluster.local:9092",
		"kafka-1.kafka-headless.devops.svc.cluster.local:9092",
		"kafka-2.kafka-headless.devops.svc.cluster.local:9092",
	}
	topic := "order"
	consumer, _ := sarama.NewConsumer(servers, nil)
	partitions, _ := consumer.Partitions(topic)
	for partition := range partitions {
		go func(partition int32) {
			pc, _ := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
			for msg := range pc.Messages() {
				m := map[string]interface{}{
					"partition": partition,
					"offset":    msg.Offset,
					"key":       msg.Key,
					"value":     fmt.Sprintf("%s", msg.Value),
				}
				b, _ := json.Marshal(m)
				logger.Info().Msg(fmt.Sprintf("%s", b))
			}
		}(int32(partition))
	}
	select {}
}

// GetLogger 获取Logger
func GetLogger() zerolog.Logger {
	return zerolog.New(
		zerolog.MultiLevelWriter(
			os.Stdout,
		),
	).With().Timestamp().Logger()
}
