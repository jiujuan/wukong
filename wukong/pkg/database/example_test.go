package database_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jiujuan/wukong/pkg/database"
)

// 使用示例
type User struct {
	ID        int64
	Name      string
	Email     string
	Age       int
	CreatedAt time.Time
}

func Example_usage() {
	// 1. 初始化连接
	cfg := database.Config{
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
		User:     "postgres",
		Password: "password",
		MaxConns: 10,
	}
	db, err := database.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	// 2. 查询单条
	var user User
	builder := database.Select("users", "id", "name", "email", "age", "created_at").
		Where("id = $1", 123)
	err = db.GetOne(ctx, &user, builder)
	if err != nil {
		log.Printf("query error: %v", err)
	}

	// 3. 条件查询列表
	listBuilder := database.Select("users", "id", "name", "email").
		Where("age > $1", 18).
		Where("status = $2", "active").
		OrderBy("created_at DESC").
		Limit(20).
		Offset(0)

	rows, err := db.List(ctx, listBuilder)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// 手动扫描结果（或封装 scan helper）
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.Name, &u.Email)
		if err != nil {
			log.Printf("scan error: %v", err)
			continue
		}
		fmt.Printf("User: %+v\n", u)
	}

	// 4. 插入数据
	insBuilder := database.Insert("users").
		Set([]string{"name", "email", "age"}, "Alice", "alice@example.com", 25)
	_, err = db.InsertOne(ctx, insBuilder)
	if err != nil {
		log.Printf("insert error: %v", err)
	}

	// 5. 更新数据
	updBuilder := database.Update("users").
		Set("age", 26).
		Where("email = $1", "alice@example.com")
	_, err = db.UpdateOne(ctx, updBuilder)

	// 6. 删除数据
	delBuilder := database.Delete("users").
		Where("id = $1", 123)
	_, err = db.DeleteOne(ctx, delBuilder)

	// 7. 事务操作
	err = db.WithTransaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		// 多个操作要么全成功，要么全回滚
		_, err := tx.Exec(ctx, "UPDATE accounts SET balance = balance - 100 WHERE user_id = $1", 1)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance + 100 WHERE user_id = $2", 2)
		return err
	})
	if err != nil {
		log.Printf("transaction failed: %v", err)
	}

	// 8. 检查存在 & 计数
	exists, _ := db.Exists(ctx, database.Select("users").Where("email = $1", "test@example.com"))
	count, _ := db.Count(ctx, "users", database.Select("users").Where("age > $1", 18))

	fmt.Printf("Exists: %v, Count: %d\n", exists, count)

	//------------- 批量插入 ---------------
	// 9. 基础批量插入（链式追加）
	sql, args, err := database.Insert("users").
		Set([]string{"name", "email", "age"}, "Alice", "alice@test.com", 25).
		AddRow("Bob", "bob@test.com", 30).
		AddRow("Charlie", "charlie@test.com", 28).
		Build()

	// 10. 批量插入（传入二维切片）
	batchrows := [][]interface{}{
		{"Dave", "dave@test.com", 22},
		{"Eve", "eve@test.com", 27},
	}
	sql, args, err = database.Insert("users").
		Set([]string{"name", "email", "age"}). // 仅设列名
		AddRows(batchrows...).
		Build()

	// 生成的 SQL (PostgreSQL 语法):
	// INSERT INTO users (name, email, age) VALUES ($1, $2, $3), ($4, $5, $6), ($7, $8, $9)
	// args: [Alice alice@test.com 25 Bob bob@test.com 30 Charlie charlie@test.com 28]

	// 最后执行插入
	db.Pool().Exec(ctx, sql, args...)
	// db.Exec(ctx, sql, args...)
}
