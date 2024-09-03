package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde/avro"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde/protobuf"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"
)

type Config struct {
	KafkaBootstrapServer string `yaml:"kafkaBootstrapServer"`
	KafkaUsername        string `yaml:"kafkaUsername,omitempty"`
	KafkaPassword        string `yaml:"kafkaPassword,omitempty"`
	KafkaGroupId         string `yaml:"kafkaGroupId"`
	SchemaRegistryKey    string `yaml:"schemaRegistryKey"`
	SchemaRegistrySecret string `yaml:"schemaRegistrySecret"`
	SchemaRegistryUrl    string `yaml:"schemaRegistryUrl"`
	OrdersProtobufTopic  string `yaml:"ordersProtobufTopic,omitempty"`
	ReviewsProtobufTopic string `yaml:"reviewsProtobufTopic,omitempty"`
	CustomersAvroTopic   string `yaml:"customersAvroTopic,omitempty"`
}

type Customer struct {
	CustomerID int    `json:"customerId"`
	Name       string `json:"name"`
	Contact    string `json:"contact"`
	Address    string `json:"address"`
	Phone      string `json:"phone"`
}

func serializeAvroMessage(client *schemaregistry.Client, topic string, msg interface{}) ([]byte, error) {
	serConf := avro.NewSerializerConfig()
	ser, err := avro.NewGenericSerializer(*client, serde.ValueSerde, serConf)
	if err != nil {
		log.Fatalf("Failed to create serializer: %s\n", err)
	}

	payload, err := ser.Serialize(topic, msg)
	if err != nil {
		log.Fatalf("Failed to serialize payload: %s\n", err)
	}

	return payload, nil
}

func serializeProtobufMessage(client *schemaregistry.Client, topic string, msg protoreflect.ProtoMessage) ([]byte, error) {
	serConf := protobuf.NewSerializerConfig()
	ser, err := protobuf.NewSerializer(*client, serde.ValueSerde, serConf)
	if err != nil {
		log.Fatalf("Failed to create serializer: %s\n", err)
	}

	payload, err := ser.Serialize(topic, msg)
	if err != nil {
		log.Fatalf("Failed to serialize payload: %s\n", err)
	}

	return payload, nil
}

func publishMessage(p *kafka.Producer, topic *string, msg []byte) error {
	err := p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: topic, Partition: kafka.PartitionAny},
		Value:          msg,
		Headers:        []kafka.Header{{Key: "ExampleHeader", Value: []byte("header values are binary")}},
	}, nil)
	if err != nil {
		log.Fatalf("Produce failed: %v", err)
	}

	e := <-p.Events()
	var m *kafka.Message
	if e != nil {
		m = e.(*kafka.Message)
	}

	if m.TopicPartition.Error != nil {
		log.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
	} else {
		log.Printf("Delivered message to topic %s [%d] at offset %v\n",
			*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
	}

	return nil
}

func main() {
	var c Config
	// Load the config file
	f, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Config file not found, please create a config file (config.yaml)!\n %s", err)
	}
	err = yaml.Unmarshal(f, &c)
	if err != nil {
		log.Fatalln(err)
	}

	kcm := kafka.ConfigMap{
		"bootstrap.servers": c.KafkaBootstrapServer,
		"group.id":          c.KafkaGroupId,
		"auto.offset.reset": "earliest",
	}

	if c.KafkaUsername != "" && c.KafkaPassword != "" {
		kcm = kafka.ConfigMap{
			"bootstrap.servers": c.KafkaBootstrapServer,
			"security.protocol": "SASL_SSL",
			"sasl.mechanism":    "PLAIN",
			"sasl.username":     c.KafkaUsername,
			"sasl.password":     c.KafkaPassword,
			"group.id":          c.KafkaGroupId,
			"auto.offset.reset": "earliest",
		}
	}

	p, err := kafka.NewProducer(&kcm)
	if err != nil {
		log.Fatalf("Failed to create producer: %s", err)
	}
	defer p.Close()

	log.Printf("Created Producer %v\n", p)

	client, err := schemaregistry.NewClient(schemaregistry.NewConfigWithAuthentication(c.SchemaRegistryUrl, c.SchemaRegistryKey, c.SchemaRegistrySecret))
	if err != nil {
		log.Fatalf("Failed to create schema registry client: %s", err)
	}

	// Use WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	if c.OrdersProtobufTopic != "" {
		// Increment the WaitGroup counter for each goroutine
		wg.Add(1)
		// Publish all orders
		log.Printf("Publishing orders to %s topic", c.OrdersProtobufTopic)
		go func() {
			// Decrement the WaitGroup conter when a goroutine finishes
			defer wg.Done()
			var rawMsgs []json.RawMessage

			of, err := os.ReadFile("data/orders.json")
			if err != nil {
				log.Fatalln(err)
			}

			if err := json.Unmarshal(of, &rawMsgs); err != nil {
				log.Fatalln(err)
			}

			for _, rMsg := range rawMsgs {
				o := Order{}
				err = protojson.Unmarshal(rMsg, &o)
				if err != nil {
					log.Fatalln(err)
				}
				// Serialize protobuf message
				pbMsg, err := serializeProtobufMessage(&client, c.OrdersProtobufTopic, &o)
				if err != nil {
					log.Fatalln(err)
				}
				// Publish message to protobuf topic
				err = publishMessage(p, &c.OrdersProtobufTopic, pbMsg)
				if err != nil {
					log.Fatalln(err)
				}
				// We will add sleep for 5s so that we can observe how the messages are being published
				time.Sleep(5 * time.Second)
			}
		}()
	}

	if c.CustomersAvroTopic != "" {
		// Increment the WaitGroup counter for each goroutine
		wg.Add(1)
		log.Printf("Publishing customers to %s topic", c.CustomersAvroTopic)
		// Publish all customers
		go func() {
			// Decrement the WaitGroup conter when a goroutine finishes
			defer wg.Done()
			var customers []Customer
			cf, err := os.ReadFile("data/customers.json")
			if err != nil {
				log.Fatalln(err)
			}
			err = json.Unmarshal(cf, &customers)
			if err != nil {
				log.Fatalln(err)
			}

			for _, msg := range customers {
				// Serialize avro message
				avroMsg, err := serializeAvroMessage(&client, c.CustomersAvroTopic, msg)
				if err != nil {
					log.Fatalln(err)
				}
				// Publish message to avro topic
				err = publishMessage(p, &c.CustomersAvroTopic, avroMsg)
				if err != nil {
					log.Fatalln(err)
				}
			}
			// We will add sleep for 5s so that we can observe how the messages are being published
			time.Sleep(5 * time.Second)
		}()
	}

	if c.ReviewsProtobufTopic != "" {
		// Increment the WaitGroup counter for each goroutine
		wg.Add(1)
		log.Printf("Publishing reviews to %s topic", c.ReviewsProtobufTopic)
		// Publish all reviews
		go func() {
			// Decrement the WaitGroup conter when a goroutine finishes
			defer wg.Done()
			var rawMsgs []json.RawMessage

			of, err := os.ReadFile("data/reviews.json")
			if err != nil {
				log.Fatalln(err)
			}

			if err := json.Unmarshal(of, &rawMsgs); err != nil {
				log.Fatalln(err)
			}

			for _, rMsg := range rawMsgs {
				o := Review{}
				err = protojson.Unmarshal(rMsg, &o)
				if err != nil {
					log.Fatalln(err)
				}
				// Serialize protobuf message
				pbMsg, err := serializeProtobufMessage(&client, c.ReviewsProtobufTopic, &o)
				if err != nil {
					log.Fatalln(err)
				}
				// Publish message to protobuf topic
				err = publishMessage(p, &c.ReviewsProtobufTopic, pbMsg)
				if err != nil {
					log.Fatalln(err)
				}
				// We will add sleep for 5s so that we can observe how the messages are being published
				time.Sleep(5 * time.Second)
			}
		}()
	}
	// Wait for all goroutines to finish
	wg.Wait()
}
