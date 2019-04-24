# ferret
Find the closest endpoint in a set of endpoints

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Demo](#demo)

## Demo
[![asciicast](https://asciinema.org/a/242651.svg)](https://asciinema.org/a/242651)

## Installation

Let's assume we are starting from scratch.

Install [go](https://golang.org/doc/install) and create a go workspace.

```sh
export GOPATH=${HOME}/go
mkdir $GOPATH
cd $GOPATH
mkdir -p src/github.com/joeabbey
cd src/github.com/joeabbey
git clone git@github.com:joeabbey/ferret
cd ferret
make
```

## Usage
```sh
ferret
```

Press "q" or "ctrl-c" to exit.  Alternatively press "r" to run again.

