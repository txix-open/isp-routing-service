### v4.0.0
* обновлены зависимости
* сервис переписан на isp-kit
* если целевой бекенд не достаупен, то вернется ошибка 502(`code.Unavailable`) вместо 501 (`code.Unimplemented`)
* добавлены логи в формате json
* таймаут на установку соединения к целевому бекенду уменьшен до 1с
### v3.1.5
* updated dependencies
* migrated to common local config
### v3.1.4
* updated dependencies
### v3.1.3
* increase max grpc body size to 64 MB for server
### v3.1.2
* updated isp-lib
### v3.1.1
* updated isp-lib
* updated isp-event-lib
### v3.1.0
* migrate to go mod
* remove pooling gprc connections
* new gprc-proxy lib
### v3.0.0
* update to new isp-lib & config service
### v2.2.0
* fix listen connect
* remove controller 
* update to new log
### v2.1.2
* add default grpc message size
### v2.1.1
* update config description
* update lib
### v2.1.0
* add default remote configuration
