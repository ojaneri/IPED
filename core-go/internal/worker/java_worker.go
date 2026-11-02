package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// IPEDWorkerConfig define os caminhos e parâmetros do worker Java.
type IPEDWorkerConfig struct {
	JavaBin   string `json:"java_bin"`
	IpedJar   string `json:"iped_jar"`
	LibDir    string `json:"lib_dir"`
	MaxHeapMB int    `json:"max_heap_mb"`
}

// IPEDWorker gerencia a invocação do IPED Lite em Java como processo filho descartável.
type IPEDWorker struct {
	cfg        IPEDWorkerConfig
	mu         sync.Mutex
	emExecucao bool
	cancelFn   context.CancelFunc
	ultimoErro string
	inicio     time.Time
	fim        time.Time
}

// NovoWorker cria uma nova instância configurada do IPEDWorker.
func NovoWorker(cfg IPEDWorkerConfig) *IPEDWorker {
	if cfg.JavaBin == "" {
		cfg.JavaBin = "java"
	}
	if cfg.MaxHeapMB <= 0 {
		cfg.MaxHeapMB = 1500
	}
	return &IPEDWorker{cfg: cfg}
}

// ConfigAutodetectar busca caminhos padrão de release do IPED no ambiente local.
func ConfigAutodetectar() IPEDWorkerConfig {
	candidatos := []string{
		`D:\iped-bench\rel-lite`,
		`D:\IPED\target\release\iped-4.4.0-SNAPSHOT`,
	}

	cfg := IPEDWorkerConfig{
		JavaBin:   "java",
		MaxHeapMB: 1500,
	}

	for _, dir := range candidatos {
		jar := filepath.Join(dir, "iped.jar")
		if _, err := os.Stat(jar); err == nil {
			cfg.IpedJar = jar
			cfg.LibDir = filepath.Join(dir, "lib")
			javaExe := filepath.Join(dir, "jre", "bin", "java.exe")
			if _, err := os.Stat(javaExe); err == nil {
				cfg.JavaBin = javaExe
			}
			break
		}
	}

	return cfg
}

// IniciarProcessamento dispara o processamento profundo em segundo plano via iped.jar.
func (w *IPEDWorker) IniciarProcessamento(evidencia, saida, perfil string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.emExecucao {
		return fmt.Errorf("já existe um processamento do IPED em andamento")
	}

	if w.cfg.IpedJar == "" {
		return fmt.Errorf("caminho do iped.jar não configurado ou não encontrado")
	}

	if perfil == "" {
		perfil = "lite"
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancelFn = cancel
	w.emExecucao = true
	w.inicio = time.Now()
	w.ultimoErro = ""

	args := []string{
		fmt.Sprintf("-Xmx%dm", w.cfg.MaxHeapMB),
		"-jar", w.cfg.IpedJar,
		"-d", evidencia,
		"-o", saida,
		"-profile", perfil,
		"--nogui",
		"--nologfile",
	}

	cmd := exec.CommandContext(ctx, w.cfg.JavaBin, args...)

	go func() {
		defer func() {
			w.mu.Lock()
			w.emExecucao = false
			w.fim = time.Now()
			w.mu.Unlock()
		}()

		out, err := cmd.CombinedOutput()
		w.mu.Lock()
		if err != nil {
			w.ultimoErro = fmt.Sprintf("falha na execução: %v (saída: %s)", err, string(out))
		}
		w.mu.Unlock()
	}()

	return nil
}

// Status retorna o estado atual do processamento Java.
func (w *IPEDWorker) Status() (bool, time.Duration, string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.emExecucao {
		return true, time.Since(w.inicio), ""
	}

	duracao := w.fim.Sub(w.inicio)
	if duracao < 0 {
		duracao = 0
	}
	return false, duracao, w.ultimoErro
}

// Cancelar interrompe o processo do IPED se estiver em execução.
func (w *IPEDWorker) Cancelar() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.emExecucao && w.cancelFn != nil {
		w.cancelFn()
	}
}
