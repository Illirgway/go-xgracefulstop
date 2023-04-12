# Go xgracefulstop package

Package for graceful stop daemon/backend app and its internal nested services

# Usage

### Simple as single package (without server)

```gotemplate
// basic usage
import "github.com/Illirgway/go-xgracefulstop"

gs := xgracefulstop.NewGS(1, xgracefulstop.DefaultTimeout)

// simple watcher start
gs.Watch()

// or with http.Server or similar
gs.

// and now wait for gs
gs.Wait()

```

#### With external service from 3rd party package 

```gotemplate
import "github.com/Illirgway/go-xgracefulstop"

// ...

func startDaemon() error {

	srv, err := somepackage.StartExternalService()

	if err != nil {
		return err
	}

	gs := xgracefulstop.NewGS(0, xgracefulstop.DefaultTimeout)

	gs.Watch()

	gs.Wait()

	srv.Stop()

	return nil
}

```

#### With internal service (contains stopChQ) 

```gotemplate
import "github.com/Illirgway/go-xgracefulstop"

// ...

func main() {

	gs, err := setup()
	
	if err != nil {
		log.Fatalln("setup:", err)
	}
	
	gs.Wait()
}

func setup() (gs *xgracefulstop.GS, err error) {

	srv, err := services.StartService()

	if err != nil {
		return nil, err
	}

	gs = xgracefulstop.NewGS(1, xgracefulstop.DefaultTimeout)
	gs.Add(srv.StopCh)

	gs.Watch()
	
	return gs, nil
}

```

### With http.Server (or embedded)

```gotemplate
import "github.com/Illirgway/go-xgracefulstop"

// ...

func main() {

	// some configuration object
	cfg, err := config.New()

	if err != nil {
		log.Fatalln("config:", err)
	}

	gs, err := setup(cfg)
	
	if err != nil {
		log.Fatalln("setup:", err)
	}

	gs.Wait()
}

func setup(cfg *config.Config) (gs *xgracefulstop.GS, err error) {

	// first setup internal services
	srv, err := services.StartService(cfg)

	if err != nil {
		return nil, err
	}

	// create http.Server and setup routers to `srv` service 
	s, err := server.CreateServer(cfg, srv)

	if err != nil {
		return nil, err
	}

	gs = xgracefulstop.NewGS(1, xgracefulstop.DefaultTimeout)
	gs.Add(srv.StopCh)
	
	gs.SetServerAndWatch(s)
	
	return gs, nil
}


```

### Complex usage with [`go-xautoserver`](https://github.com/Illirgway/go-xautoserver)

```gotemplate
import ( 
	"github.com/Illirgway/go-xautoserver"
	"github.com/Illirgway/go-xgracefulstop"
)

// ...

func main() {

	// some configuration object
	cfg, err := config.New()

	if err != nil {
		log.Fatalln("config:", err)
	}

	h, gs, err := setup(cfg)

	if err != nil {
		log.Fatalln("setup:", err)
	}

	if err = startGSServer(cfg, h, gs); err != nil {
		log.Fatalln("server:", err)
	}

	gs.Wait()
}

func setup(cfg *config.Config) (h http.Handler, gs *xgracefulstop.GS, err error) {

	// first setup internal services
	srv, err := services.StartService(cfg)

	if err != nil {
		return nil, nil, err
	}

	// create http.Server and setup routers to `srv` service 
	h, err = server.SetupHttpHandler(cfg, srv)

	if err != nil {
		return nil, nil, err
	}

	gs = xgracefulstop.NewGS(1, xgracefulstop.DefaultTimeout)
	gs.Add(srv.StopCh)
	
	return h, gs, nil
}

func startGSServer(cfg *config.Config, h http.Handler, gs *xgracefulstop.GS) error {

	var tls *xautoserver.TLSConfig = nil

	// let's say cfg.GetTLS() returns key and cert file paths (abs path strings)
	// and cfg.GetListen() returns listen address as for http.ListenAndServe

	cert, key := cfg.GetTLS()

	if cert != "" && key != "" {
		tls = &xautoserver.TLSConfig{
			Cert: cert,
			Key: key,
		}
	}

	srvCfg := xautoserver.Config{
		// server-side timeouts
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,

		MaxHeaderBytes:	1 << 20,

		TLS:	tls,

		// important part - use xautoserver.Config callback function to pass new http srv to GS 
		SrvCb:	gs.SetServerAndWatch,	 
	}

	return xautoserver.ListenAndServe(cfg.GetListen(), h, &srvCfg)
}

```