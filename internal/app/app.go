package app

import (
	"context"
	"log/slog"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)
type App struct{DB *store.DB;Logger *slog.Logger}
func New(db *store.DB,l *slog.Logger)*App{if l==nil{l=slog.Default()};return &App{DB:db,Logger:l}}
func(a *App)Settings(ctx context.Context)(store.Settings,error){return a.DB.GetSettings(ctx)}
func(a *App)SaveSettings(ctx context.Context,s store.Settings)error{return a.DB.SaveSettings(ctx,s)}
