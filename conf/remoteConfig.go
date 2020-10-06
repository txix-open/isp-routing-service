package conf

import (
	"time"

	"github.com/integration-system/isp-lib/v2/structure"
)

const (
	KB = int64(1024)
	MB = int64(1 << 20)

	defaultSyncTimeout   = 30 * time.Second
	defaultStreamTimeout = 60 * time.Second

	defaultMaxRequestBodySize  = 512 * MB
	defaultBufferSize          = 4 * KB
	DefaultMaxResponseBodySize = 32 * MB
)

type RemoteConfig struct {
	Metrics     structure.MetricConfiguration `schema:"Настройка метрик"`
	HttpSetting HttpSetting                   `schema:"Настройка сервера"`
	GrpcSetting GrpcSetting                   `schema:"Настройка grpc соединения"`
}

type HttpSetting struct {
	MaxRequestBodySizeBytes int64 `schema:"Максимальный размер тела запроса,в байтайх, по умолчанию: 512 MB"`
}

func (cfg HttpSetting) GetMaxRequestBodySize() int64 {
	if cfg.MaxRequestBodySizeBytes <= 0 {
		return defaultMaxRequestBodySize
	}
	return cfg.MaxRequestBodySizeBytes
}

type GrpcSetting struct {
	EnableOriginalProtoErrors            bool  `schema:"Проксирование ошибок в протобаф,включение/отключение проксирования, по умолчанию отключено"`
	ProxyGrpcErrorDetails                bool  `schema:"Проксирование первого элемента из details GRPC ошибки,включение/отключение проксирования, по умолчанию отключено"`
	MultipartDataTransferBufferSizeBytes int64 `schema:"Размер буфера для передачи бинарных файлов,по умолчанию 4 KB"`
	SyncInvokeMethodTimeoutMs            int64 `schema:"Время ожидания вызова метода,значение в миллисекундах, по умолчанию: 30000"`
	StreamInvokeMethodTimeoutMs          int64 `schema:"Время ожидания передачи и обработки файла,значение в миллисекундах, по умолчанию: 60000"`
}

func (cfg GrpcSetting) GetSyncInvokeTimeout() time.Duration {
	if cfg.SyncInvokeMethodTimeoutMs <= 0 {
		return defaultSyncTimeout
	}
	return time.Duration(cfg.SyncInvokeMethodTimeoutMs) * time.Millisecond
}

func (cfg GrpcSetting) GetStreamInvokeTimeout() time.Duration {
	if cfg.StreamInvokeMethodTimeoutMs <= 0 {
		return defaultStreamTimeout
	}
	return time.Duration(cfg.StreamInvokeMethodTimeoutMs) * time.Millisecond
}

func (cfg GrpcSetting) GetTransferFileBufferSize() int64 {
	if cfg.MultipartDataTransferBufferSizeBytes <= 0 {
		return defaultBufferSize
	}
	return cfg.MultipartDataTransferBufferSizeBytes
}
