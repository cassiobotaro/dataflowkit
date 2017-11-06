package storage

import (
	slug "github.com/slotix/slugifyurl"
	"github.com/spf13/viper"
)

type Store interface {
	//Reads value from storage by specified key
	Read(key string) (value []byte, err error)
	//Writes specified pair key value to storage.
	//expTime value sets TTL for Redis storage.
	//expTime set Metadata Expires value for S3Storage
	Write(key string, value []byte, expTime int64) error
}

type Type string

const (
	S3    Type = "S3"
	Diskv      = "Diskv"
	Redis      = "Redis"
)

func NewStore(t Type) Store {
	switch t {
	case Diskv:
		baseDir := viper.GetString("DISKV_BASE_DIR")
		return newDiskvStorage(baseDir, 1024*1024)
	case S3:
		bucket := viper.GetString("FETCH_BUCKET")
		return newS3Storage(bucket)
	case Redis:
		redisHost := viper.GetString("REDIS")
		redisPassword := ""
		return newRedisStorage(redisHost, redisPassword)
	default:
		return nil
	}
}

func newRedisStorage(redisHost, redisPassword string) Store {
	redisCon := NewRedisConn(redisHost, redisPassword, "", 0)
	return redisCon
}

func (s RedisConn) Read(key string) (value []byte, err error) {
	value, err = s.GetValue(key)
	return
}

func (s RedisConn) Write(key string, value []byte, expTime int64) error {
	err := s.SetValue(key, value)
	if err != nil {
		return err
	}
	err = s.SetExpireAt(key, expTime)
	if err != nil {
		return err
	}
	return nil
}

func newS3Storage(bucket string) Store {
	s3Conn := newS3Conn(bucket)
	return s3Conn
}

func (s S3Conn) Read(key string) (value []byte, err error) {
	value, err = s.Download(key)
	return
}

func (s S3Conn) Write(key string, value []byte, expTime int64) error {
	err := s.Upload(key, value, expTime)
	if err != nil {
		return err
	}
	return nil
}

func newDiskvStorage(baseDir string, CacheSizeMax uint64) Store {
	d := newDiskvConn(baseDir, CacheSizeMax)
	return d
}

func (d DiskvConn) Read(key string) (value []byte, err error) {

	//Slugify key/URL to a sanitized string before reading.
	sKey := slug.Slugify(key, d.options)
	value, err = d.diskv.Read(sKey)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (d DiskvConn) Write(key string, value []byte, expTime int64) error {

	//Slugify key/URL to a sanitized string before writing.
	sKey := slug.Slugify(key, d.options)
	err := d.diskv.Write(sKey, value)
	if err != nil {
		return err
	}
	return nil
}
