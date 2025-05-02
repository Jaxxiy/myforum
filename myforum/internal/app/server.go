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

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/gorilla/mux"
	"github.com/jaxxiy/myforum/internal/handlers"
	"github.com/jaxxiy/myforum/internal/repository"
	"github.com/jaxxiy/myforum/internal/services"
	"google.golang.org/grpc"
)

type Server struct {
	httpServer *http.Server
	grpcServer *grpc.Server
	db         *repository.Postgres
	wg         sync.WaitGroup
}

func NewServer() *Server {
	r := mux.NewRouter()

	// Строка подключения к базе данных
	dsn := "postgres://postgres:Stas2005101010!@localhost:5432/forum?sslmode=disable"

	// Создаем подключение к базе
	db, err := repository.NewPostgres(dsn)
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		log.Fatalf("Не удалось создать драйвер миграций: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://../../migrations", // путь к папке с миграциями
		"forum",                   // имя БД
		driver,
	)
	if err != nil {
		log.Fatalf("Ошибка инициализации миграций: %v", err)
	}

	// Применяем миграции
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Ошибка применения миграций: %v", err)
	}

	/*
		forumRepo := repository.NewForumsRepo(db.DB)
		handlers.RegisterForumHandlers(r, forumRepo)

		// 2. Затем статические файлы (CSS, JS, изображения)
		//r.PathPrefix("/static/").Handler(
		//http.StripPrefix("/static/",
		//	http.FileServer(http.Dir("static"))))

		// 3. HTML-шаблоны (отдельный обработчик)
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Рендерим главную страницу
			tmpl, err := template.ParseFiles("templates/base.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, nil)
		})

		// 4. Fallback для SPA (если используете фронтенд)
		r.PathPrefix("/").Handler(http.FileServer(http.Dir("frontend")))

		httpSrv := &http.Server{
			Addr:    ":8080",
			Handler: r,
		}
	*/

	// Создаем репозиторий форумов
	forumRepo := repository.NewForumsRepo(db.DB) // предполагается, что db.DB это *sql.DB

	// Регистрация API-хендлеров с передачей репозитория
	handlers.RegisterForumHandlers(r, forumRepo)

	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("C:/Users/Soulless/Desktop/myforum/cmd/frontend/"))))

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	grpcSrv := grpc.NewServer()

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
