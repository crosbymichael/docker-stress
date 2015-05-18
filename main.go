package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var (
	counter   int
	failCount int
)

type Image struct {
	Name    string   `json:"name"`
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
	counter++
	p := "-P=false"
	if i.Publish {
		p = "-P=true"
	}
	cmd := exec.Command(w.binary, append([]string{"run", p, "--rm", i.Name}, i.Args...)...)
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
		failCount++
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
		if counter > max {
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
		secounds := time.Now().Sub(start).Seconds()
		logrus.Infof("ran %d containers in %f seconds (%f per sec.)", counter, secounds, float64(counter)/secounds)
		logrus.Infof("failures %d", failCount)
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
