package rxgo

type justIterable struct {
	items []any
	opts  []Option
}

func newJustIterable(items ...any) func(opts ...Option) Iterable {
	return func(opts ...Option) Iterable {
		return &justIterable{
			items: items,
			opts:  opts,
		}
	}
}

func (i *justIterable) Observe(opts ...Option) <-chan Item {
	option := parseOptions(append(i.opts, opts...)...)
	next := option.buildChannel()

	go SendItems(option.buildContext(emptyContext), next, CloseChannel, i.items)
	return next
}
