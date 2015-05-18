# docker-stress
Simple go stress test for docker

```bash
NAME:
   stress - stress test your docker daemon

USAGE:
   stress [global options] command [command options] [arguments...]

VERSION:
   0.0.0

AUTHOR(S): 
   
COMMANDS:
   help, h      Shows a list of commands or help for one command
   
GLOBAL OPTIONS:
   --binary, -b "docker"        path to the docker binary to test
   --config "stress.json"       path to the stress test configuration
   --concurrent, -c "1"         number of concurrent workers to run
   --containers "1000"          number of containers to run
   --kill, -k "10s"             time to kill a container after an execution
   --help, -h                   show help
   --version, -v                print the version

```
