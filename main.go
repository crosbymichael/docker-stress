package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

var (
	concurrent int
	duration   time.Duration
	killTime   time.Duration
	binary     string

	counter int
)

type Image struct {
	Name    string   `json:"name"`
	Args    []string `json:"args"`
	Publish bool     `json:"publish"`
	Kill    bool     `json:"kill"`
}

func init() {
	flag.IntVar(&concurrent, "concurrent", 1, "number of concurrent workers to run")

	flag.DurationVar(&duration, "duration", 10*time.Minute, "duration to run the tests")
	flag.DurationVar(&killTime, "kill", 10*time.Second, "time to kill a container after")

	flag.StringVar(&binary, "binary", "docker", "path to docker binary")

	flag.Parse()
}

func run(i *Image) {
	counter++

	p := "-P=false"
	if i.Publish {
		p = "-P=true"
	}

	cmd := exec.Command(binary, append([]string{"run", p, "--rm", i.Name}, i.Args...)...)
	if i.Kill {
		go func() {
			<-time.After(killTime)
			cmd.Process.Signal(syscall.SIGTERM)
		}()
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(out))
	}
}

func worker(group *sync.WaitGroup, c chan *Image) {
	defer group.Done()

	for i := range c {
		run(i)
	}
}

func loadImages() ([]*Image, error) {
	f, err := os.Open("stress.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var images []*Image
	if err := json.NewDecoder(f).Decode(&images); err != nil {
		return nil, err
	}
	return images, nil
}

func process(images []*Image, c chan *Image) {
	after := time.After(duration)
	for {
		for _, i := range images {
			select {
			case <-after:
				close(c)
				return
			default:
				c <- i
			}
		}
	}
}

func main() {
	var (
		c     = make(chan *Image, concurrent)
		group = &sync.WaitGroup{}
		start = time.Now()
	)

	images, err := loadImages()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < concurrent; i++ {
		group.Add(1)
		go worker(group, c)
	}

	go process(images, c)

	group.Wait()

	secounds := time.Now().Sub(start).Seconds()

	log.Printf("ran %d containers in %f seconds (%f per sec.)\n", counter, secounds, float64(counter)/secounds)
}
