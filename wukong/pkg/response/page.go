package response

// PageReq 分页请求
type PageReq struct {
	Page  int    `form:"page" json:"page"`
	Size  int    `form:"size" json:"size"`
	Sort  string `form:"sort" json:"sort"`
	Order string `form:"order" json:"order"`
}

// Option 函数选项模式
type PageOption func(*PageReq)

// WithPage 设置页码
func WithPage(page int) PageOption {
	return func(p *PageReq) {
		p.Page = page
	}
}

// WithSize 设置每页大小
func WithSize(size int) PageOption {
	return func(p *PageReq) {
		p.Size = size
	}
}

// WithSort 设置排序字段
func WithSort(sort string) PageOption {
	return func(p *PageReq) {
		p.Sort = sort
	}
}

// WithOrder 设置排序方向
func WithOrder(order string) PageOption {
	return func(p *PageReq) {
		p.Order = order
	}
}

// NewPageReq 创建分页请求
func NewPageReq(opts ...PageOption) *PageReq {
	p := &PageReq{
		Page:  1,
		Size:  10,
		Sort:  "created_at",
		Order: "desc",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Validate 验证分页参数
func (p *PageReq) Validate() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Size < 1 {
		p.Size = 10
	}
	if p.Size > 100 {
		p.Size = 100
	}
	if p.Order != "asc" && p.Order != "desc" {
		p.Order = "desc"
	}
}

// PageResp 分页响应
type PageResp struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
	Pages int         `json:"pages"`
}

// NewPageResp 创建分页响应
func NewPageResp(list interface{}, total int64, page, size int) *PageResp {
	pages := int(total) / size
	if int(total)%size != 0 {
		pages++
	}
	return &PageResp{
		List:  list,
		Total: total,
		Page:  page,
		Size:  size,
		Pages: pages,
	}
}
