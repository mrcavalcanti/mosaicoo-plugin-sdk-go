package backend

import (
	"github.com/mosaicoo/mosaicoo-plugin-sdk-go/backend/log"
)

// Logger is the default logger instance. This can be used directly to log from
// your plugin to mosaicoo-server with calls like backend.Logger.Debug(...).
var Logger = log.DefaultLogger
