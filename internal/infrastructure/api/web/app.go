package web

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/sessions"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/address"
	"github.com/v8tix/beecore-store-v2/internal/core/auth"
	"github.com/v8tix/beecore-store-v2/internal/core/basket"
	"github.com/v8tix/beecore-store-v2/internal/core/order"
	"github.com/v8tix/beecore-store-v2/internal/core/payment"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/core/product"
	"github.com/v8tix/beecore-store-v2/internal/core/user"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/api/web/handler"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/addressremote"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/authremote"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/basketremote"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/orderremote"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/payphoneremote"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/productremote"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/userremote"
	redissession "github.com/v8tix/beecore-store-v2/internal/infrastructure/redis/session"
)

// app holds this composition root's own cross-cutting dependencies —
// everything middleware.go's authenticate/requireAuthenticatedUser and
// session.go's currentSession/refreshIfNeeded need that isn't specific to
// one vertical slice's handler, plus the handlers themselves for
// routes.go to mount. Mirrors the source repo's application struct in
// cmd/web/main.go, minus the fields (repo, a package-local session.Store)
// now split across ports/adapters. PayPal is not represented anywhere here
// (spec assumption #5) — no strategy-selection field, no PayPal handler.
type app struct {
	cfg          *config.Cfg
	logger       *slog.Logger
	authRemote   resource.AuthRemote
	sessionStore resource.SessionStore
	cookieStore  *sessions.CookieStore

	authHandler    *handler.AuthHandler
	userHandler    *handler.UserHandler
	addressHandler *handler.AddressHandler
	productHandler *handler.ProductHandler
	basketHandler  *handler.BasketHandler
	orderHandler   *handler.OrderHandler
	paymentHandler *handler.PaymentHandler

	wg sync.WaitGroup
}

// resources are every vertical slice's outbound adapter, each implementing
// its slice's port/resource interface — this app's analog of the
// reference's repositories struct. No PayPalRemote (spec assumption #5).
type resources struct {
	authRemote     resource.AuthRemote
	userRemote     resource.UserRemote
	addressRemote  resource.AddressRemote
	productRemote  resource.ProductRemote
	basketRemote   resource.BasketRemote
	orderRemote    resource.OrderRemote
	payPhoneRemote resource.PayPhoneRemote
	sessionStore   resource.SessionStore
}

// services are every vertical slice's use-case that has a route mounted on
// it in routes.go.
type services struct {
	auth    service.Auth
	user    service.User
	address service.Address
	product service.Product
	basket  service.Basket
	order   service.Order
	payment service.Payment
}

// handlers are every vertical slice's chi handler that has a route in
// routes.go.
type handlers struct {
	auth    *handler.AuthHandler
	user    *handler.UserHandler
	address *handler.AddressHandler
	product *handler.ProductHandler
	basket  *handler.BasketHandler
	order   *handler.OrderHandler
	payment *handler.PaymentHandler
}

func setupResources(cfg *config.Cfg) *resources {
	return &resources{
		authRemote:     authremote.NewClient(cfg),
		userRemote:     userremote.NewClient(cfg),
		addressRemote:  addressremote.NewClient(cfg),
		productRemote:  productremote.NewClient(cfg),
		basketRemote:   basketremote.NewClient(cfg),
		orderRemote:    orderremote.NewClient(cfg),
		payPhoneRemote: payphoneremote.NewClient(cfg),
		sessionStore:   redissession.NewStore(config.NewRedisTokenCache(cfg.Redis.Client)),
	}
}

// sessionTTLFromCfg mirrors application.sessionTTL in the source repo's
// cmd/web/helpers.go — resolved once here (app.sessionTTL, session.go,
// delegates to it) rather than read from cfg afresh at every call site the
// way the source repo did, since every caller here (services, handlers,
// the CSRF cookie's MaxAge) needs the exact same value.
func sessionTTLFromCfg(cfg *config.Cfg) time.Duration {
	if cfg.Session.MaxAge <= 0 {
		return 7 * 24 * time.Hour
	}
	return time.Duration(cfg.Session.MaxAge) * time.Second
}

func setupServices(cfg *config.Cfg, res *resources) (*services, error) {
	accessTokenDuration, err := cfg.JWT.GetDuration()
	if err != nil {
		return nil, err
	}

	sessionTTL := sessionTTLFromCfg(cfg)

	return &services{
		auth: auth.NewAuthService(auth.Dependencies{
			AuthRemote:          res.authRemote,
			SessionStore:        res.sessionStore,
			AddressRemote:       res.addressRemote,
			AdminEmail:          cfg.Admin.Email,
			AdminPassword:       cfg.Admin.Password,
			BuyerRoleID:         cfg.UserRoles.StoreBuyerID,
			AccessTokenDuration: accessTokenDuration,
			SessionTTL:          sessionTTL,
		}),
		user: user.NewUserService(user.Dependencies{
			UserRemote: res.userRemote,
			AuthRemote: res.authRemote,
		}),
		address: address.NewAddressService(address.Dependencies{
			AddressRemote: res.addressRemote,
			AuthRemote:    res.authRemote,
		}),
		product: product.NewProductService(product.Dependencies{
			ProductRemote: res.productRemote,
			AuthRemote:    res.authRemote,
		}),
		basket: basket.NewBasketService(basket.Dependencies{
			BasketRemote: res.basketRemote,
			AuthRemote:   res.authRemote,
		}),
		order: order.NewOrderService(order.Dependencies{
			OrderRemote: res.orderRemote,
			AuthRemote:  res.authRemote,
		}),
		payment: payment.NewPaymentService(payment.Dependencies{
			PayPhoneRemote: res.payPhoneRemote,
			AuthRemote:     res.authRemote,
			OrderRemote:    res.orderRemote,
			BasketRemote:   res.basketRemote,
		}),
	}, nil
}

func setupHandlers(cfg *config.Cfg, res *resources, svc *services) *handlers {
	sessionTTL := sessionTTLFromCfg(cfg)
	currency := cfg.BusinessParameters.Country.Currency.Iso4217.Alpha
	taxesPercentage := cfg.BusinessParameters.Country.Taxes.Percentage

	return &handlers{
		auth: handler.NewAuthHandler(svc.auth, svc.basket, cfg.Session.SessionStore, cfg.Logger),
		user: handler.NewUserHandler(
			svc.user, svc.basket, res.sessionStore, cfg.Session.SessionStore, sessionTTL, cfg.Logger,
		),
		address: handler.NewAddressHandler(
			svc.address, res.sessionStore, cfg.Session.SessionStore, sessionTTL, cfg.Logger,
		),
		product: handler.NewProductHandler(svc.product, cfg.Session.SessionStore, cfg.Logger),
		basket:  handler.NewBasketHandler(svc.basket, res.sessionStore, cfg.Session.SessionStore, cfg.Logger),
		order:   handler.NewOrderHandler(svc.order, res.sessionStore, cfg.Session.SessionStore, cfg.Logger),
		payment: handler.NewPaymentHandler(
			svc.payment, svc.basket, svc.address, res.sessionStore, currency, taxesPercentage, cfg.Session.SessionStore, cfg.Logger,
		),
	}
}

func setupApp(cfg *config.Cfg) (*app, error) {
	res := setupResources(cfg)

	svc, err := setupServices(cfg, res)
	if err != nil {
		return nil, err
	}

	h := setupHandlers(cfg, res, svc)

	return &app{
		cfg:          cfg,
		logger:       cfg.Logger,
		authRemote:   res.authRemote,
		sessionStore: res.sessionStore,
		cookieStore:  cfg.Session.SessionStore,

		authHandler:    h.auth,
		userHandler:    h.user,
		addressHandler: h.address,
		productHandler: h.product,
		basketHandler:  h.basket,
		orderHandler:   h.order,
		paymentHandler: h.payment,
	}, nil
}

// Start builds the composition root and runs the HTTP server until it's
// asked to shut down. Mirrors run in the source repo's cmd/web/main.go,
// minus the config load/cfg.InitSelected call, which stays in cmd/web/main.go
// — cfg arrives here already fully initialized.
func Start(cfg *config.Cfg) error {
	a, err := setupApp(cfg)
	if err != nil {
		return err
	}

	return a.serveHTTP(cfg)
}
