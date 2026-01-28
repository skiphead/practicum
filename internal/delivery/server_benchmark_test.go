// server_benchmark_test.go
package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// 1. Базовый бенчмарк chi роутера (самый важный)
func BenchmarkChiRouter(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		}
	})
}

// 2. Бенчмарк параметризованных маршрутов
func BenchmarkChiRouterWithParams(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		_ = id // Используем переменную
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"` + id + `"}`))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		}
	})
}

// 3. Бенчмарк middleware цепочки
func BenchmarkChiRouterWithMiddleware(b *testing.B) {
	router := chi.NewRouter()

	// Добавляем middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-1", "true")
			next.ServeHTTP(w, r)
		})
	})

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-2", "true")
			next.ServeHTTP(w, r)
		})
	})

	router.Get("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/api/test", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		}
	})
}

// 4. УПРОЩЕННЫЙ бенчмарк HTTP запросов (без исчерпания сокетов)
func BenchmarkHTTPDirectHandler(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	req := httptest.NewRequest("GET", "/api/data", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		}
	})
}

// 5. Бенчмарк различных HTTP методов
func BenchmarkHTTPMethods(b *testing.B) {
	router := chi.NewRouter()

	router.Get("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.Post("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	router.Put("/resource/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.Delete("/resource/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	b.Run("GET", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/resource", nil)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
			}
		})
	})

	b.Run("POST", func(b *testing.B) {
		req := httptest.NewRequest("POST", "/resource", nil)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
			}
		})
	})

	b.Run("PUT", func(b *testing.B) {
		req := httptest.NewRequest("PUT", "/resource/123", nil)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
			}
		})
	})

	b.Run("DELETE", func(b *testing.B) {
		req := httptest.NewRequest("DELETE", "/resource/123", nil)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
			}
		})
	})
}

// 6. Бенчмарк выделения памяти
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	router := chi.NewRouter()
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}
}

// 7. Бенчмарк вложенных маршрутов
func BenchmarkNestedRoutes(b *testing.B) {
	router := chi.NewRouter()

	router.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})
	})

	paths := []string{
		"/api/v1/users",
		"/api/v1/users/123",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			path := paths[counter%len(paths)]
			counter++

			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		}
	})
}

// 8. Бенчмарк обработки заголовков
func BenchmarkRequestWithHeaders(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/api/headers", func(w http.ResponseWriter, r *http.Request) {
		// Читаем заголовки
		contentType := r.Header.Get("Content-Type")
		auth := r.Header.Get("Authorization")
		_ = contentType + auth // Используем переменные

		w.Header().Set("X-Processed", "true")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"headers_received":true}`))
	})

	req := httptest.NewRequest("GET", "/api/headers", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		}
	})
}
