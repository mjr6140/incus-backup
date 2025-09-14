package main

import (
    "os"

    "incus-backup/src/cli"
)

func main() {
    code := cli.Execute()
    os.Exit(code)
}

