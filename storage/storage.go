package storage

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/sirupsen/logrus"
	"github.com/slotix/dataflowkit/log"
	"github.com/spf13/viper"
)

var logger *logrus.Logger

func init() {
	logger = log.NewLogger()
}

//Store is the key interface of storage. All other structs implement methods wchich satisfy that interface.
type Store interface {
	//Reads value from storage by specified key
	Read(key string) (value []byte, err error)
	//Writes specified pair key value to storage.
	//expTime value sets TTL for Redis storage.
	//expTime set Metadata Expires value for S3Storage
	Write(key string, value []byte, expTime int64) error
	//Is key expired ? It checks if parse results storage item is expired. Set up  Expiration as "ITEM_EXPIRE_IN" environment variable.
	//html pages cache stores this info in sResponse.Expires . It is not used for fetch endpoint.
	Expired(key string) bool
	//Delete deletes specified item from the store
	Delete(key string) error
	//DeleteAll erases all items from the store
	DeleteAll() error
}

//Type represent available storage types
type Type string

const (
	//Amazon S3 storage
	S3 Type = "S3"
	//Digital Ocean Spaces
	Spaces = "Spaces"
	//diskv key/value storage "github.com/peterbourgon/diskv"
	Diskv = "Diskv"
	//Redis
	Redis = "Redis"
)

// ParseType takes a string representing storage type and returns the Storage Type constant.
func ParseType(t string) *Type {
	var tp Type
	switch strings.ToLower(t) {
	case "s3":
		tp = S3
	case "spaces":
		tp = Spaces
	case "diskv":
		tp = Diskv
	case "redis":
		tp = Redis
	default:
		return nil
	}
	return &tp
}

// NewStore creates New initialized Store instance with predefined parameters
func NewStore(t Type) Store {
	switch t {
	case Diskv:
		baseDir := viper.GetString("DISKV_BASE_DIR")
		return newDiskvStorage(baseDir, 1024*1024)
	case S3: //AWS S3
		bucket := viper.GetString("DFK_BUCKET")
		config := &aws.Config{
			Region: aws.String(viper.GetString("S3_REGION")),
		}
		return newS3Storage(config, bucket)

	case Spaces: //Digital Ocean Spaces
		bucket := viper.GetString("DFK_BUCKET")
		config := &aws.Config{
			Credentials: credentials.NewSharedCredentials(viper.GetString("SPACES_CONFIG"), ""), //Load credentials from specified file
			Endpoint:    aws.String(viper.GetString("SPACES_ENDPOINT")),                         //Endpoint is obligatory for DO Spaces
			Region:      aws.String(viper.GetString("S3_REGION")),
			//Region:      aws.String("ams333"),                                                   //Actually for Digital Ocean spaces region parameter may have any value. But it can't be omitted.
		}
		return newS3Storage(config, bucket)

	case Redis:
		redisHost := viper.GetString("REDIS")
		redisPassword := ""
		return newRedisStorage(redisHost, redisPassword)
	default:
		return nil
	}
}

func newRedisStorage(redisHost, redisPassword string) Store {
	redisCon := NewRedisConn()
	return redisCon
}

func newS3Storage(config *aws.Config, bucket string) Store {
	s3Conn := newS3Conn(config, bucket)
	return s3Conn
}

func newDiskvStorage(baseDir string, CacheSizeMax uint64) Store {
	d := newDiskvConn(baseDir, CacheSizeMax)
	return d
}
