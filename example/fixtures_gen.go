package example

func ptr[T any](v T) *T { return &v }

func FixtureUser() User {
	return User{
		ID: "UserID",
		FirstName: "FirstName",
		LastName: "LastName",
		Email: "Email",
		Age: 1,
		Active: true,
		Address: ptr(FixtureAddress()),
		Tags: []string{"Tags"},
	}
}

func FixtureAddress() Address {
	return Address{
		Street: "Street",
		City: "City",
		Country: "Country",
		ZipCode: "ZipCode",
	}
}


