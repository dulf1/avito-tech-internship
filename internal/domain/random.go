package domain

type RandomSource interface {
	Shuffle(n int, swap func(i, j int))
}
