package ldap

import (
	"log"
	"testing"
)

func Test_LDAPClient(t *testing.T) {
	t.Run("Authenticate", func(t *testing.T) {
		client := &LDAPClient{
			Base:         "dc=example,dc=com",
			Host:         "ldap.example.com",
			Port:         389,
			UseSSL:       false,
			BindDN:       "uid=readonlysuer,ou=People,dc=example,dc=com",
			BindPassword: "readonlypassword",
			UserFilter:   "(uid=%s)",
			GroupFilter:  "(memberUid=%s)",
			Attributes:   []string{"givenName", "sn", "mail", "uid"},
		}
		defer client.Close()

		ok, user, err := client.Authenticate("username", "password")
		if err != nil {
			log.Fatalf("Error authenticating user %s: %+v", "username", err)
		}
		if !ok {
			log.Fatalf("Authenticating failed for user %s", "username")
		}
		log.Printf("User: %+v", user)
	})

	t.Run("GetGroupsOfUser", func(t *testing.T) {
		client := &LDAPClient{
			Base:        "dc=example,dc=com",
			Host:        "ldap.example.com",
			Port:        389,
			GroupFilter: "(memberUid=%s)",
		}
		defer client.Close()
		groups, err := client.GetGroupsOfUser("username")
		if err != nil {
			log.Fatalf("Error getting groups for user %s: %+v", "username", err)
		}
		log.Printf("Groups: %+v", groups)
	})

	t.Run("RunQueries", func(t *testing.T) {
		client := &LDAPClient{
			Base:         "ou=People,dc=planetexpress,dc=com",
			Host:         "localhost",
			Port:         389,
			UseSSL:       false,
			BindDN:       "cn=admin,dc=planetexpress,dc=com",
			BindPassword: "GoodNewsEveryone",
			UserFilter:   "(uid=%s)",
			GroupFilter:  "(memberUid=%s)",
			Attributes:   []string{"givenName", "sn", "mail", "uid"},
		}
		defer client.Close()

		ok, user, err := client.Authenticate("fry", "fry")
		if err != nil {
			log.Fatalf("Error authenticating user %s: %+v", "username", err)
		}
		if !ok {
			log.Fatalf("Authenticating failed for user %s", "username")
		}
		log.Printf("User: %+v", user)

		query := "(ou=Office Management)"
		attribute := "cn"
		results, err := client.RunQueries("fry", []string{"(ou=Delivering Crew)", "(ou=Ship Crew)"})
		if err != nil {
			log.Fatalf("Error searching query %s and attribute %s: %+v", query, attribute, err)
		}

		log.Printf("found: %+v", results)
	})

	t.Run("GetAllGroupsByName", func(t *testing.T) {
		client := &LDAPClient{
			Base:         "ou=People,dc=planetexpress,dc=com",
			Host:         "127.0.0.1",
			Port:         389,
			UseSSL:       false,
			BindDN:       "cn=admin,dc=planetexpress,dc=com",
			BindPassword: "GoodNewsEveryone",
			UserFilter:   "(uid=%s)",
			GroupFilter:  "(memberUid=%s)",
			Attributes:   []string{"givenName", "sn", "mail", "uid"},
		}
		defer client.Close()

		results, err := client.GetAllGroupsByName("ship_crew")
		if err != nil {
			log.Fatalf("Error getting all groups  matching to %s, %v", "ship_crew", err)
		}

		log.Printf("found: %+v", results)
	})

	t.Run("ChangeUserPassword", func(t *testing.T) {
		client := &LDAPClient{
			Base:               "DC=trial,DC=local",
			Host:               "34.244.56.18",
			Port:               636,
			UseSSL:             true,
			SkipTLS:            false,
			InsecureSkipVerify: true,
			BindDN:             "CN=mike-t,DC=trial,DC=local",
			BindPassword:       "Aa123456#",
			UserFilter:         "(sAMAccountName=%s)",
			GroupFilter:        "(gid=%s)",
			Attributes:         []string{"sAMAccountName", "uid"},
		}
		defer client.Close()

		oldPassword, newPassword := "Aa123456#", "Vv123456#"
		err := client.ChangeUserPassword("mike-t", oldPassword, newPassword)
		if err != nil {
			log.Fatalf("Error changing user password from %s to %s, %v", oldPassword, newPassword, err)
		}
	})
}
