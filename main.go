package main

import (
	"log"
	"net/http"

	"github.com/luizaranda/go-core/pkg/app"
	loglevel "github.com/luizaranda/go-core/pkg/log"
	"github.com/luizaranda/go-core/pkg/web"
)

/*
* Example to run a web application with go-core
 */
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	webApp, err := NewApplication()
	if err != nil {
		return err
	}

	webApp.RegisterModule(NewEndpoint())

	return webApp.Run()
}

type Module interface {
	Bind(app *Application)
}

type Application struct {
	*app.Application
}

func NewApplication() (*Application, error) {
	opts := []app.AppOptFunc{
		app.WithLogLevel(loglevel.DebugLevel),
		app.WithEnableProfiling(),
	}
	webApp, err := app.NewWebApplication(opts...)
	if err != nil {
		return nil, err
	}
	return &Application{
		Application: webApp,
	}, nil
}

func (app *Application) RegisterModule(module Module) *Application {
	module.Bind(app)
	return app
}

type Endpoint struct {
}

func NewEndpoint() *Endpoint {
	return &Endpoint{}
}

func (e *Endpoint) Bind(app *Application) {
	app.Get("/hello", func(w http.ResponseWriter, r *http.Request) error {
		return web.EncodeJSON(w, "Hello World", http.StatusOK)
	})
}
