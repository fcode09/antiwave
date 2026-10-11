.PHONY: build run gen demo release tidy vet test clean

# Compila el binario (para el SO actual) en ./antiwave
build:
	go build -o antiwave ./cmd/antiwave

# Compila y ejecuta con un archivo de audio
# Uso: make run AUDIO=ruta/al/archivo.wav
run: build
	./antiwave $(AUDIO)

# Captura el audio del sistema y lo visualiza
capture: build
	./antiwave

# Genera un WAV demo (arpegio de notas) en samples/demo.wav
gen:
	go run ./cmd/genwav -o samples/demo.wav

# Genera el demo y lo reproduce
demo: gen
	go run ./cmd/antiwave samples/demo.wav

# Compila binarios estáticos para Windows y Linux en dist/
release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/antiwave.exe ./cmd/antiwave
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/antiwave     ./cmd/antiwave

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf antiwave dist samples/demo.wav
