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

type UserReference struct {
	Id      isUserReference_Id
	Version string
}

type isUserReference_Id interface {
	isUserReference_Id()
}

type UserReference_EmailId struct {
	EmailId string
}

func (UserReference_EmailId) isUserReference_Id() {}

type UserReference_UserId struct {
	UserId string
}

func (UserReference_UserId) isUserReference_Id() {}

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
