package models

import (
	"encoding/json"
	"testing"
)

func TestNewCart(t *testing.T) {
	c := NewCart()
	if c.Count() != 0 {
		t.Errorf("new cart count = %d, want 0", c.Count())
	}
	if c.Total() != 0 {
		t.Errorf("new cart total = %f, want 0", c.Total())
	}
}

func TestCartAdd(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00})
	c.Add(CartItem{ProductID: 2, Title: "Bowl", Price: 30.00})

	if c.Count() != 2 {
		t.Errorf("count = %d, want 2", c.Count())
	}
}

func TestCartAdd_DuplicatePrevention(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00})
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00})

	if c.Count() != 1 {
		t.Errorf("count = %d, want 1 (duplicate should be rejected)", c.Count())
	}
}

func TestCartRemove(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00})
	c.Add(CartItem{ProductID: 2, Title: "Bowl", Price: 30.00})

	c.Remove(1)
	if c.Count() != 1 {
		t.Errorf("count = %d, want 1 after remove", c.Count())
	}
	if c.Items[0].ProductID != 2 {
		t.Errorf("remaining item ID = %d, want 2", c.Items[0].ProductID)
	}
}

func TestCartRemove_NonExistent(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00})
	c.Remove(999) // should not panic or change count
	if c.Count() != 1 {
		t.Errorf("count = %d, want 1 after removing non-existent", c.Count())
	}
}

func TestCartClear(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00})
	c.Add(CartItem{ProductID: 2, Title: "Bowl", Price: 30.00})

	c.Clear()
	if c.Count() != 0 {
		t.Errorf("count = %d, want 0 after clear", c.Count())
	}
}

func TestCartTotal(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.50})
	c.Add(CartItem{ProductID: 2, Title: "Bowl", Price: 30.00})

	total := c.Total()
	if total != 55.50 {
		t.Errorf("total = %f, want 55.50", total)
	}
}

func TestCartTotal_Empty(t *testing.T) {
	c := NewCart()
	if c.Total() != 0 {
		t.Errorf("empty cart total = %f, want 0", c.Total())
	}
}

func TestCartMarshal(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00, Thumbnail: "mug.jpg"})

	data, err := c.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var items []CartItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ProductID != 1 {
		t.Errorf("product_id = %d, want 1", items[0].ProductID)
	}
}

func TestCartMarshal_Empty(t *testing.T) {
	c := NewCart()
	data, err := c.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if data != "[]" {
		t.Errorf("empty marshal = %q, want []", data)
	}
}

func TestCartFromJSON(t *testing.T) {
	jsonData := `[{"product_id":1,"title":"Mug","price":25.00,"thumbnail":"mug.jpg"},{"product_id":2,"title":"Bowl","price":30.00,"thumbnail":""}]`

	c := CartFromJSON(jsonData)
	if c.Count() != 2 {
		t.Errorf("count = %d, want 2", c.Count())
	}
	if c.Items[0].Title != "Mug" {
		t.Errorf("first item title = %q, want Mug", c.Items[0].Title)
	}
	if c.Total() != 55.00 {
		t.Errorf("total = %f, want 55.00", c.Total())
	}
}

func TestCartFromJSON_Empty(t *testing.T) {
	c := CartFromJSON("")
	if c.Count() != 0 {
		t.Errorf("count = %d, want 0 for empty string", c.Count())
	}
}

func TestCartFromJSON_InvalidJSON(t *testing.T) {
	c := CartFromJSON("not-json")
	// Should return empty cart without panic
	if c == nil {
		t.Fatal("expected non-nil cart for invalid JSON")
	}
}

func TestCartRoundTrip(t *testing.T) {
	c := NewCart()
	c.Add(CartItem{ProductID: 1, Title: "Mug", Price: 25.00, Thumbnail: "mug.jpg"})
	c.Add(CartItem{ProductID: 2, Title: "Bowl", Price: 30.00, Thumbnail: "bowl.jpg"})

	data, err := c.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	c2 := CartFromJSON(data)
	if c2.Count() != c.Count() {
		t.Errorf("round-trip count: got %d, want %d", c2.Count(), c.Count())
	}
	if c2.Total() != c.Total() {
		t.Errorf("round-trip total: got %f, want %f", c2.Total(), c.Total())
	}
}
