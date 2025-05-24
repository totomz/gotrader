package gotrader

//
// import (
// 	"context"
// 	"fmt"
// 	"log/slog"
// 	"os"
// )
//
// func SetDefaultLogger() {
// 	slog.SetDefault(slog.New(NewCtxLogHandler()))
// }
//
// type CtxLogHandler struct {
// 	next slog.Handler
// }
//
// func NewCtxLogHandler() *CtxLogHandler {
// 	return &CtxLogHandler{
// 		next: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
// 			// // Remove time.
// 			// if a.Key == slog.TimeKey && len(groups) == 0 {
// 			// 	return slog.Attr{}
// 			// }
//
// 			if a.Key == slog.LevelKey && len(groups) == 0 {
// 				a.Value = slog.StringValue(fmt.Sprintf("%s", a.Value))
// 			}
//
// 			if a.Key == slog.SourceKey {
// 				source := a.Value.Any().(*slog.Source)
// 				source.File = shortenFilePath(source.File)
// 			}
// 			return a
// 		}}),
// 	}
// }
//
// func (h *CtxLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
// 	return true
// }
//
// func (h *CtxLogHandler) Handle(ctx context.Context, record slog.Record) error {
// 	candle := ctx.Value(candleCtxKey)
// 	if candle != nil {
// 		c, err := candle.(Candle)
// 		if err {
// 			panic("invalid candle in slog!")
// 		}
//
// 		record.AddAttrs(slog.String("ticker", string(c.Symbol)))
// 		record.AddAttrs(slog.String("inst", c.TimeStr()))
// 	}
//
// 	return h.next.Handle(ctx, record)
// }
//
// // WithAttrs returns a new handler with the provided attributes.
// func (h *CtxLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
// 	return &CtxLogHandler{
// 		next: h.next.WithAttrs(attrs),
// 	}
// }
//
// // WithGroup returns a new handler with the provided group name.
// func (h *CtxLogHandler) WithGroup(name string) slog.Handler {
// 	return &CtxLogHandler{
// 		next: h.next.WithGroup(name),
// 	}
// }
//
// func shortenFilePath(path string) string {
//
// 	return path
// 	// if len(path) < 9 {
// 	// 	return path
// 	// }
// 	// if !strings.Contains(path, "/services") {
// 	// 	return path
// 	// }
// 	// parts := strings.Split(path, "/services")
// 	// return "services" + parts[1]
// }
