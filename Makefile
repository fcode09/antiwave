.PHONY: build run gen demo tidy vet test clean

# Compila el binario en ./antiwave
build:
	go build -o antiwave ./cmd/antiwave

# Compila y ejecuta con un archivo de audio
# Uso: make run AUDIO=ruta/al/archivo.wav
run: build
	./antiwave $(AUDIO)

# Genera un WAV demo (arpegio de notas) en samples/demo.wav
gen:
	go run ./cmd/genwav -o samples/demo.wav

# Genera el demo y lo reproduce
demo: gen
	go run ./cmd/antiwave samples/demo.wav

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f antiwave samples/demo.wav
