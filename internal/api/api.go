package api

import (
	"github.com/alexedwards/scs/v2"
	"github.com/felipefardo/gobid/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Api struct {
	Router         *chi.Mux
	UserService    services.UserService
	ProductService services.ProductService
	Sessions       *scs.SessionManager
	WsUpgrader     websocket.Upgrader
	AuctionLobby   services.AuctionLobby
}
