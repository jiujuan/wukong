package response

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("RequestID", "test-request-id")

	Success(c, gin.H{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body should not be empty")
	}
}

func TestFail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("RequestID", "test-request-id")

	Fail(c, 400, "bad request")

	if w.Code != http.StatusOK { // 注意：Fail返回200但code是400
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body should not be empty")
	}
}

func TestResponseOptions(t *testing.T) {
	r := New(
		WithCode(200),
		WithMsg("success"),
		WithData(gin.H{"key": "value"}),
		WithRequestID("test-id"),
	)

	if r.Code != 200 {
		t.Errorf("Code = %d, want %d", r.Code, 200)
	}
	if r.Msg != "success" {
		t.Errorf("Msg = %v, want %v", r.Msg, "success")
	}
	if r.RequestID != "test-id" {
		t.Errorf("RequestID = %v, want %v", r.RequestID, "test-id")
	}
}

func TestNewPageReq(t *testing.T) {
	p := NewPageReq()
	if p.Page != 1 {
		t.Errorf("Default Page = %d, want %d", p.Page, 1)
	}
	if p.Size != 10 {
		t.Errorf("Default Size = %d, want %d", p.Size, 10)
	}
	if p.Sort != "created_at" {
		t.Errorf("Default Sort = %v, want %v", p.Sort, "created_at")
	}
	if p.Order != "desc" {
		t.Errorf("Default Order = %v, want %v", p.Order, "desc")
	}
}

func TestNewPageReqWithOptions(t *testing.T) {
	p := NewPageReq(
		WithPage(5),
		WithSize(20),
		WithSort("updated_at"),
		WithOrder("asc"),
	)

	if p.Page != 5 {
		t.Errorf("Page = %d, want %d", p.Page, 5)
	}
	if p.Size != 20 {
		t.Errorf("Size = %d, want %d", p.Size, 20)
	}
	if p.Sort != "updated_at" {
		t.Errorf("Sort = %v, want %v", p.Sort, "updated_at")
	}
	if p.Order != "asc" {
		t.Errorf("Order = %v, want %v", p.Order, "asc")
	}
}

func TestPageReqValidate(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		size     int
		order    string
		wantPage int
		wantSize int
	}{
		{"negative page", -1, 10, "desc", 1, 10},
		{"zero page", 0, 10, "desc", 1, 10},
		{"size too large", 1, 200, "desc", 1, 100},
		{"invalid order", 1, 10, "invalid", 1, 10},
		{"valid", 5, 50, "asc", 5, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PageReq{Page: tt.page, Size: tt.size, Order: tt.order}
			p.Validate()

			if p.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", p.Page, tt.wantPage)
			}
			if p.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", p.Size, tt.wantSize)
			}
		})
	}
}

func TestNewPageResp(t *testing.T) {
	list := []string{"a", "b", "c"}
	resp := NewPageResp(list, 25, 1, 10)

	if resp.Total != 25 {
		t.Errorf("Total = %d, want %d", resp.Total, 25)
	}
	if resp.Page != 1 {
		t.Errorf("Page = %d, want %d", resp.Page, 1)
	}
	if resp.Size != 10 {
		t.Errorf("Size = %d, want %d", resp.Size, 10)
	}
	if resp.Pages != 3 { // 25/10 = 2.5, should round up to 3
		t.Errorf("Pages = %d, want %d", resp.Pages, 3)
	}
}

func TestPageRespExactDivision(t *testing.T) {
	list := []string{}
	resp := NewPageResp(list, 20, 1, 10)

	if resp.Pages != 2 { // 20/10 = 2
		t.Errorf("Pages = %d, want %d", resp.Pages, 2)
	}
}
