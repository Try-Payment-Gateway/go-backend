package main

import (
	"log"
	"net/http"
	"payment_backend/internal/config"
	httpd "payment_backend/internal/delivery/http"
	"payment_backend/internal/repository"
	"payment_backend/internal/usecase"
)

func main() {
	cfg := config.Load()

	repo, err := repository.NewSQLiteRepo(cfg.SQLiteDSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer repo.Close()

	uc := usecase.NewQRUsecase(repo)
	h := httpd.NewHandler(uc, repo)

	r := h.Routes(httpd.SigConfig{
		Secret:        cfg.HMACSecret,
		MaxAgeSeconds: cfg.SigMaxAgeSeconds,
	})

	addr := ":" + cfg.AppPort
	log.Printf("Server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
