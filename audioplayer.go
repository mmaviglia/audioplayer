package audioplayer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"sync"

	"github.com/hajimehoshi/oto"
)

var ffmpegPath string = "ffmpeg"

// SetFFmpegPath sets the path to the FFmpeg executable. This should be used if FFmpeg is not
// in the system PATH.
func SetFFmpegPath(path string) {
	ffmpegPath = path
}

type AudioPlayer struct {
	mu sync.Mutex
	wg sync.WaitGroup

	cancelPlayback context.CancelFunc
	ffmpegCmd      *exec.Cmd
	player         *oto.Player
	context        *oto.Context
	audioFile      string

	speed     float64 // must be between 0.5 and 2.0
	startTime int     // in seconds
}

func NewAudioPlayer(audioFile string, speed float64, startTime int) *AudioPlayer {
	return &AudioPlayer{
		audioFile: audioFile,
		speed:     speed,
		startTime: startTime,
	}
}

// Start creates an FFmpeg process to decode the audio file into raw PCM data and
// creates an Oto player to play the audio. This should be called to start audio playback.
func (ap *AudioPlayer) Start() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.ffmpegCmd != nil {
		ap.mu.Unlock()
		ap.Close()
		ap.mu.Lock()
	}

	// FFmpeg command to decode audio into raw PCM
	ffmpegCmd := exec.Command(
		ffmpegPath,
		"-ss", strconv.Itoa(ap.startTime),
		"-i", ap.audioFile,
		"-filter:a", "atempo="+strconv.FormatFloat(ap.speed, 'f', -1, 64),
		"-f", "s16le",
		"-ar", "44100",
		"-ac", "2",
		"pipe:1",
	)

	ffmpegOut, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create FFmpeg stdout pipe: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ap.cancelPlayback = cancel
	reader := NewReader(ctx, ffmpegOut)

	if err := ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("start FFmpeg cmd: %w", err)
	}

	// Create Oto context and player if not already initialized
	if ap.context == nil {
		bufferSizeInBytes := 4 * 1024

		context, err := oto.NewContext(44100, 2, 2, bufferSizeInBytes)
		if err != nil {
			return fmt.Errorf("create oto context: %w", err)
		}
		ap.context = context
	}

	ap.wg.Add(1)
	ap.player = ap.context.NewPlayer()
	go func() {
		if _, err := io.Copy(ap.player, reader); err != nil {
			log.Printf("Error copying audio data: %v", err)
		}
		ap.wg.Done()
		ap.Stop()
	}()

	ap.ffmpegCmd = ffmpegCmd
	return nil
}

// Stop closes any active oto.Player and kills any active FFmpeg process. This should be
// called to pause or stop audio playback.
func (ap *AudioPlayer) Stop() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	// Cancel the context to stop the goroutine
	if ap.cancelPlayback != nil {
		ap.cancelPlayback()
	}

	// Wait for the goroutine to finish
	ap.wg.Wait()

	if ap.ffmpegCmd != nil {
		_ = ap.ffmpegCmd.Process.Kill()
		_ = ap.ffmpegCmd.Wait()
		ap.ffmpegCmd = nil
	}

	if ap.player != nil {
		ap.player.Close()
		ap.player = nil
	}
}

// Close stops the audio player and closes the oto.Context.
func (ap *AudioPlayer) Close() {
	ap.Stop()
	if ap.context != nil {
		ap.context.Close()
		ap.context = nil
	}
}

type readerCtx struct {
	ctx context.Context
	r   io.Reader
}

// Read is the Read method of the io.Reader interface.
func (r *readerCtx) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// NewReader returns a context-aware io.Reader.
func NewReader(ctx context.Context, r io.Reader) io.Reader {
	return &readerCtx{ctx: ctx, r: r}
}
