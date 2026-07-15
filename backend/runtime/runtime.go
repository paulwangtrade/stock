package runtimeutil

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func Emit(ctx context.Context,event string,data interface{}) {

	if ctx == nil {
		return
	}

	runtime.EventsEmit(ctx,event,data)
}