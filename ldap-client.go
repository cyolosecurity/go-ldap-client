// Package ldap provides a simple ldap client to authenticate,
// retrieve basic information and groups for a user.
package ldap

import (
	"crypto/tls"
	"fmt"
	"golang.org/x/text/encoding/unicode"
	"gopkg.in/ldap.v2"
)

// adPasswordAttribute is the password attribute name in active directory.
const adPasswordAttributeName = "UnicodePwd"

var (
	// errUserNotExist represents a situation in which the user object doesn't exist.
	errUserNotExist = fmt.Errorf("user does not exist")
	// errUserNotIdentified represents a situation in which the user could not be identified.
	errUserNotIdentified = fmt.Errorf("user could not be indentifed")
)

type LDAPClient struct {
	Attributes         []string
	Base               string
	BindDN             string
	BindPassword       string
	GroupFilter        string // e.g. "(memberUid=%s)"
	Host               string
	ServerName         string
	UserFilter         string // e.g. "(uid=%s)"
	Conn               *ldap.Conn
	Port               int
	InsecureSkipVerify bool
	UseSSL             bool
	SkipTLS            bool
	ClientCertificates []tls.Certificate // Adding client certificates
}

type LdapGroup struct {
	Name              string
	DistinguishedName string
}

// Connect connects to the ldap backend.
func (lc *LDAPClient) Connect() error {
	if lc.Conn == nil {
		var l *ldap.Conn
		var err error
		address := fmt.Sprintf("%s:%d", lc.Host, lc.Port)
		if !lc.UseSSL {
			l, err = ldap.Dial("tcp", address)
			if err != nil {
				return err
			}

			// Reconnect with TLS
			if !lc.SkipTLS {
				err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
				if err != nil {
					return err
				}
			}
		} else {
			config := &tls.Config{
				InsecureSkipVerify: lc.InsecureSkipVerify,
				ServerName:         lc.ServerName,
			}
			if lc.ClientCertificates != nil && len(lc.ClientCertificates) > 0 {
				config.Certificates = lc.ClientCertificates
			}
			l, err = ldap.DialTLS("tcp", address, config)
			if err != nil {
				return err
			}
		}

		lc.Conn = l
	}
	return nil
}

// Close closes the ldap backend connection.
func (lc *LDAPClient) Close() {
	if lc.Conn != nil {
		lc.Conn.Close()
		lc.Conn = nil
	}
}

// Authenticate authenticates the user against the ldap backend.
func (lc *LDAPClient) Authenticate(username, password string) (bool, map[string][]string, error) {
	err := lc.Connect()
	if err != nil {
		return false, nil, err
	}

	// First bind with a read only user
	if lc.BindDN != "" && lc.BindPassword != "" {
		err := lc.Conn.Bind(lc.BindDN, lc.BindPassword)
		if err != nil {
			return false, nil, err
		}
	}

	attributes := append(lc.Attributes, "dn")
	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		lc.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(lc.UserFilter, username),
		attributes,
		nil,
	)

	sr, err := lc.Conn.Search(searchRequest)
	if err != nil {
		return false, nil, err
	}

	if len(sr.Entries) < 1 {
		return false, nil, errUserNotExist
	}

	if len(sr.Entries) > 1 {
		return false, nil, errUserNotIdentified
	}

	userDN := sr.Entries[0].DN
	user := map[string][]string{}
	for _, attr := range lc.Attributes {
		user[attr] = sr.Entries[0].GetAttributeValues(attr)
	}

	// Bind as the user to verify their password
	err = lc.Conn.Bind(userDN, password)
	if err != nil {
		return false, user, err
	}

	// Rebind as the read only user for any further queries
	if lc.BindDN != "" && lc.BindPassword != "" {
		err = lc.Conn.Bind(lc.BindDN, lc.BindPassword)
		if err != nil {
			return true, user, err
		}
	}

	return true, user, nil
}

// RunQueries runs the given ldap queries against the ldap backend and returns the matched queries.
func (lc *LDAPClient) RunQueries(username string, queries []string) (results map[string]bool, err error) {
	if err = lc.Connect(); err != nil {
		return
	}

	// First bind with a read only user
	if lc.BindDN != "" && lc.BindPassword != "" {
		if err = lc.Conn.Bind(lc.BindDN, lc.BindPassword); err != nil {
			return
		}
	}

	results = map[string]bool{}

	attributes := []string{"dn"}
	userFilter := fmt.Sprintf(lc.UserFilter, username)

	for _, query := range queries {
		searchRequest := ldap.NewSearchRequest(
			lc.Base,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			fmt.Sprintf("(&%s%s)", userFilter, query),
			attributes,
			nil,
		)

		var sr *ldap.SearchResult
		if sr, err = lc.Conn.Search(searchRequest); err != nil {
			return
		}

		if len(sr.Entries) == 1 {
			results[query] = true
		}
	}

	return
}

// GetGroupsOfUser returns the group for a user.
func (lc *LDAPClient) GetGroupsOfUser(username string) ([]string, error) {
	err := lc.Connect()
	if err != nil {
		return nil, err
	}

	searchRequest := ldap.NewSearchRequest(
		lc.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(lc.GroupFilter, username),
		[]string{"cn"}, // can it be something else than "cn"?
		nil,
	)
	sr, err := lc.Conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}
	var groups []string
	for _, entry := range sr.Entries {
		groups = append(groups, entry.GetAttributeValue("cn"))
	}
	return groups, nil
}

// GetAllGroupsByName returns list of groups matching a name.
func (lc *LDAPClient) GetAllGroupsByName(groupName string) ([]LdapGroup, error) {
	err := lc.Connect()
	if err != nil {
		return nil, err
	}

	// First bind with a read only user
	if lc.BindDN != "" && lc.BindPassword != "" {
		if err = lc.Conn.Bind(lc.BindDN, lc.BindPassword); err != nil {
			return nil, nil
		}
	}

	searchRequest := ldap.NewSearchRequest(
		lc.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=Group)(cn=*%s*))", groupName),
		[]string{"cn"},
		nil,
	)

	sr, err := lc.Conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}
	var groups []LdapGroup
	for _, entry := range sr.Entries {
		group := LdapGroup{
			Name:              entry.GetAttributeValue("cn"),
			DistinguishedName: entry.DN,
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// ChangeUserPassword changes user's password.
// Note - currently this function is relevant only to Active Directory.
func (lc *LDAPClient) ChangeUserPassword(username, oldPassword, newPassword string) (err error) {
	err = lc.Connect()
	if err != nil {
		return
	}

	// first bind with a read only user
	if lc.BindDN != "" && lc.BindPassword != "" {
		if err = lc.Conn.Bind(lc.BindDN, lc.BindPassword); err != nil {
			err = fmt.Errorf("could not bind read only user: %w", err)
			return
		}
	}

	attributes := append(lc.Attributes, "dn")
	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		lc.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(lc.UserFilter, username),
		attributes,
		nil,
	)

	sr, err := lc.Conn.Search(searchRequest)
	if err != nil {
		return err
	}

	if len(sr.Entries) < 1 {
		return errUserNotExist
	}

	if len(sr.Entries) > 1 {
		return errUserNotIdentified
	}

	userDN := sr.Entries[0].DN
	user := map[string][]string{}
	for _, attr := range lc.Attributes {
		user[attr] = sr.Entries[0].GetAttributeValues(attr)
	}

	// bind as the user to verify their current password
	if err = lc.Conn.Bind(userDN, oldPassword); err != nil {
		err = fmt.Errorf("could not bind user via old password: %w", err)
		return
	}

	var oldEncodedPass, newEncodedPass string

	if oldEncodedPass, err = lc.encodePasswordForAD(oldPassword); err != nil {
		return
	}
	if newEncodedPass, err = lc.encodePasswordForAD(newPassword); err != nil {
		return
	}

	modify := ldap.NewModifyRequest(userDN)
	modify.Delete(adPasswordAttributeName, []string{oldEncodedPass})
	modify.Add(adPasswordAttributeName, []string{newEncodedPass})
	if err = lc.Conn.Modify(modify); err != nil {
		err = fmt.Errorf("password could not be changed: %w", err)
		return
	}

	// bind as the user to verify their new password
	if err = lc.Conn.Bind(userDN, newPassword); err != nil {
		err = fmt.Errorf("could not bind user via new password: %w", err)
		return
	}

	return
}

// encodePasswordForAD encodes the password for active directory.
func (lc *LDAPClient) encodePasswordForAD(pass string) (encoded string, err error) {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	if encoded, err = utf16.NewEncoder().String(fmt.Sprintf("%q", pass)); err != nil {
		err = fmt.Errorf("password could not be utf16 encoded: %w", err)
	}
	return
}
