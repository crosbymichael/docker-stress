package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var (
	counter  int
	failures int
	mutex    = &sync.Mutex{}
)

type Image struct {
	Name    string   `json:"name"`
	Flags   []string `json:"flags"`
	Args    []string `json:"args"`
	Publish bool     `json:"publish"`
	Kill    bool     `json:"kill"`
}

func newWorker(binary string, kill time.Duration, group *sync.WaitGroup) *worker {
	return &worker{
		binary:   binary,
		killTime: kill,
		wg:       group,
	}
}

type worker struct {
	binary   string
	killTime time.Duration
	wg       *sync.WaitGroup
}

func (w *worker) start(c chan *Image) {
	defer w.wg.Done()
	for i := range c {
		w.run(i)
	}
}

func (w *worker) run(i *Image) {
	mutex.Lock()
	counter++
	mutex.Unlock()
	p := "-P=false"
	if i.Publish {
		p = "-P=true"
	}

	command := []string{"run", p, "--rm"}
	if len(i.Flags) > 0 {
		flags := []string{}

		for _, f := range i.Flags {
			if f != "--rm" && !strings.HasPrefix(f, "-P") && !strings.HasPrefix(f, "--publish") {
				flags = append(flags, f)
			}
		}

		command = append(command, flags...)
	}
	command = append(command, i.Name)
	command = append(command, i.Args...)
	cmd := exec.Command(w.binary, command...)
	if i.Kill {
		go func() {
			<-time.After(w.killTime)
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				logrus.Error(err)
			}
		}()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		mutex.Lock()
		failures++
		mutex.Unlock()
		logrus.WithField("error", err).Errorf("%s", out)
	}
}

func loadImages(path string) ([]*Image, error) {
	f, err := os.Open(path)
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

func process(images []*Image, c chan *Image, max int) {
	for {
		mutex.Lock()
		completed := counter > max
		mutex.Unlock()
		if completed {
			close(c)
			return
		}
		for _, i := range images {
			c <- i
		}
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "stress"
	app.Usage = "stress test your docker daemon"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "binary,b", Value: "docker", Usage: "path to the docker binary to test"},
		cli.StringFlag{Name: "config", Value: "stress.json", Usage: "path to the stress test configuration"},
		cli.IntFlag{Name: "concurrent,c", Value: 1, Usage: "number of concurrent workers to run"},
		cli.IntFlag{Name: "containers", Value: 1000, Usage: "number of containers to run"},
		cli.DurationFlag{Name: "kill,k", Value: 10 * time.Second, Usage: "time to kill a container after an execution"},
	}
	app.Action = func(context *cli.Context) {
		var (
			c     = make(chan *Image, context.GlobalInt("concurrent"))
			group = &sync.WaitGroup{}
			start = time.Now()
		)
		images, err := loadImages(context.GlobalString("config"))
		if err != nil {
			logrus.Fatal(err)
		}
		for i := 0; i < context.GlobalInt("concurrent"); i++ {
			group.Add(1)
			w := newWorker(context.GlobalString("binary"), context.GlobalDuration("kill"), group)
			go w.start(c)
		}
		go process(images, c, context.GlobalInt("containers"))
		group.Wait()
		seconds := time.Now().Sub(start).Seconds()
		logrus.Infof("ran %d containers in %f seconds (%f per sec.)", counter, seconds, float64(counter)/seconds)
		logrus.Infof("failures %d", failures)
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
