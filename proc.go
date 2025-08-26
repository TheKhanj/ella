package main

import (
	"context"
)

type Proc interface {
	Run(ctx context.Context) error
}
