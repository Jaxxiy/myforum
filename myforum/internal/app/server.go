package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/jaxxiy/myforum/internal/handlers"
	"github.com/jaxxiy/myforum/internal/repository"
	"github.com/jaxxiy/myforum/internal/services"
	"google.golang.org/grpc"
)

type Server struct {
	httpServer *http.Server
	grpcServer *grpc.Server
	db         *repository.Postgres // подключение к базе
	wg         sync.WaitGroup
}

func NewServer() *Server {
	r := mux.NewRouter()

	// Регистрация API-хендлеров
	handlers.RegisterForumHandlers(r)

	// Обслуживание статических файлов (например, index.html)
	// Предположим, что папка frontend находится в корне проекта
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("C:/Users/Soulless/Desktop/myforum/cmd/frontend/"))))

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	grpcSrv := grpc.NewServer()

	// Строка подключения к базе данных
	dsn := "postgres://postgres:Stas2005101010!@localhost:5432/forum?sslmode=disable"

	// Создаем подключение к базе
	db, err := repository.NewPostgres(dsn)
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	// Запуск WebSocket
	go services.StartWebSocket()

	return &Server{
		httpServer: httpSrv,
		grpcServer: grpcSrv,
		db:         db,
	}
}

func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// gRPC
	ln, err := net.Listen("tcp", ":9090")
	if err != nil {
		return err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.grpcServer.Serve(ln); err != nil {
			log.Printf("gRPC остановился: %v", err)
		}
	}()

	// HTTP
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP остановился: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Завершение работы...")
	s.grpcServer.GracefulStop()
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		return err
	}
	s.wg.Wait()
	return nil
}
