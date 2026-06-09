package dto

type Item interface {
	GetKey() string
	GetValue() any
}

type ItemImpl struct {
	Key   string
	Value any
}

func (i *ItemImpl) GetKey() string {
	return i.Key
}

func (i *ItemImpl) SetKey(key string) {
	i.Key = key
}

func (i *ItemImpl) GetValue() any {
	return i.Value
}

func (i *ItemImpl) SetValue(value any) {
	i.Value = value
}

type ItemOnce interface {
	GetKey() string
	GetValue() func() (any, error)
}

type ItemOnceImpl struct {
	Key string
	Fn  func() (any, error)
}

func (i *ItemOnceImpl) GetKey() string {
	return i.Key
}

func (i *ItemOnceImpl) SetKey(key string) {
	i.Key = key
}

func (i *ItemOnceImpl) SetValue(fn func() (any, error)) {
	i.Fn = fn
}

func (i *ItemOnceImpl) GetValue() func() (any, error) {
	return i.Fn
}
