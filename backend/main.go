package main

import (
	"backend/cmd"
)

func main() {
    api := cmd.NewAPI(cmd.APIConfig{
        Addr: ":8080",
    })
    api.Mount()
    api.Run()
}
