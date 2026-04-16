package models

import (
	"encoding/json"
	"sync"
)

type CartItem struct {
	ProductID int64   `json:"product_id"`
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	Thumbnail string  `json:"thumbnail"`
}

type Cart struct {
	Items []CartItem `json:"items"`
	mu    sync.Mutex
}

func NewCart() *Cart {
	return &Cart{Items: []CartItem{}}
}

func (c *Cart) Add(item CartItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Don't add duplicates (each pottery piece is unique)
	for _, existing := range c.Items {
		if existing.ProductID == item.ProductID {
			return
		}
	}
	c.Items = append(c.Items, item)
}

func (c *Cart) Remove(productID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, item := range c.Items {
		if item.ProductID == productID {
			c.Items = append(c.Items[:i], c.Items[i+1:]...)
			return
		}
	}
}

func (c *Cart) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Items = []CartItem{}
}

func (c *Cart) Total() float64 {
	total := 0.0
	for _, item := range c.Items {
		total += item.Price
	}
	return total
}

func (c *Cart) Count() int {
	return len(c.Items)
}

func (c *Cart) Marshal() (string, error) {
	b, err := json.Marshal(c.Items)
	return string(b), err
}

func CartFromJSON(data string) *Cart {
	c := NewCart()
	if data == "" {
		return c
	}
	json.Unmarshal([]byte(data), &c.Items)
	return c
}
