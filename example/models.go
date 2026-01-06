package example

import (
	"google.golang.org/protobuf/types/known/timestamppb"
)

type User struct {
	ID         string
	FirstName  string
	LastName   string
	Email      string
	Age        int
	Active     bool
	Address    *Address
	Tags       []string
	CreatedAt  *timestamppb.Timestamp
	Activities []*Activity
}

type ItemVersionReference struct {
	Id      isItemVersionReference_Id
	Version string
}

type isItemVersionReference_Id interface {
	isItemVersionReference_Id()
}

type ItemVersionReference_OseonId struct {
	OseonId int64
}

func (ItemVersionReference_OseonId) isItemVersionReference_Id() {}

type ItemVersionReference_SmosId struct {
	SmosId string
}

func (ItemVersionReference_SmosId) isItemVersionReference_Id() {}

type Activity struct {
	Name        string
	Description string
}

type Address struct {
	Street  string
	City    string
	Country string
	ZipCode string
}
