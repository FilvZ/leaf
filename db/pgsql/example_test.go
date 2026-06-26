package pgsql_test

import (
	"fmt"
	"github.com/name5566/leaf/db/pgsql"
)

func Example() {
	c, err := pgsql.Dial("postgres://localhost:5432/test?sslmode=disable", 10)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer c.Close()

	// ensure sequence for auto-increment
	err = c.EnsureSequence("user_id_seq")
	if err != nil {
		fmt.Println(err)
		return
	}

	// get next auto-increment id
	for i := 0; i < 3; i++ {
		id, err := c.NextSeq("user_id_seq")
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(id)
	}

	// ensure table
	err = c.EnsureTable("players", "id BIGSERIAL PRIMARY KEY, name TEXT, level INT DEFAULT 1")
	if err != nil {
		fmt.Println(err)
		return
	}

	// ensure unique index
	err = c.EnsureUniqueIndex("players", []string{"name"})
	if err != nil {
		fmt.Println(err)
		return
	}

	// insert a record
	_, err = c.Exec("INSERT INTO players (name, level) VALUES ($1, $2)", "leaf", 99)
	if err != nil {
		fmt.Println(err)
		return
	}

	// query
	var name string
	var level int
	err = c.QueryRow("SELECT name, level FROM players WHERE name = $1", "leaf").Scan(&name, &level)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(name, level)

	// cleanup
	_ = c.DropTable("players")
	_ = c.DropSequence("user_id_seq")

	// Output:
	// 1
	// 2
	// 3
	// leaf 99
}