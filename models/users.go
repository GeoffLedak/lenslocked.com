package models

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"lenslocked.com/hash"
	"lenslocked.com/rand"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"golang.org/x/crypto/bcrypt"
)

const (
	// ErrNotFound is returned when a resource cannot be found in the database.
	ErrNotFound modelError = "models: resource not found"

	// ErrIDInvalid is returned when an invalid ID is provided to a method like Delete.
	ErrIDInvalid modelError = "models: ID provided was invalid"

	// ErrPasswordIncorrect is returned when an invalid password is used when attempting to authenticate a user.
	ErrPasswordIncorrect modelError = "models: incorrect password provided"

	// ErrEmailRequired is returned when an email address is not provided when creating a user
	ErrEmailRequired modelError = "models: email address is required"

	// ErrEmailInvalid is returned when an email address provided does not match any of our requirements
	ErrEmailInvalid modelError = "models: email address is not valid"

	// ErrEmailTaken is returned when an update or create is attempted with an email address that is already in use.
	ErrEmailTaken modelError = "models: email address is already taken"

	// ErrPasswordTooShort is returned when a user tries to set a password that is less than 8 characters long.
	ErrPasswordTooShort modelError = "models: password must be at least 8 characters long"

	// ErrPasswordRequired is returned when a create is attempted without a user password provided.
	ErrPasswordRequired modelError = "models: password is required"

	// ErrRememberRequired is returned when a create or update is attempted without a user remember token hash
	ErrRememberRequired modelError = "models: remember token is required"

	// ErrRememberTooShort is returned when a remember token is not at least 32 bytes
	ErrRememberTooShort modelError = "models: remember token must be at least 32 bytes"
)

// UserDB is used to interact with the users database.
//
// For pretty much all single user queries:
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with
// more information about what went wrong. This may not be
// an error generated by the models package.
//
// For single user queries, any error but ErrNotFound should
// probably result in a 500 error until we make "public"
// facing errors.
type UserDB interface {
	// Methods for querying for single users
	ByID(id string) (*User, error)
	ByEmail(email string) (*User, error)
	ByRemember(token string) (*User, error)

	// Methods for altering users
	Create(user *User) error
	Update(user *User) error
	Delete(id string) error
}

type User struct {
	ID           primitive.ObjectID `bson:"_id"`
	Name         string             `bson:"name"`
	Email        string             `bson:"email"`
	Password     string             `bson:"password"`
	PasswordHash string             `bson:"passwordHash"`
	Remember     string             `bson:"remember"`
	RememberHash string             `bson:"rememberHash"`
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

// UserService is a set of methods used to manipulate and
// work with the user model
type UserService interface {
	// Authenticate will verify the provided email address and
	// password are correct. If they are correct, the user
	// corresponding to that email will be returned. Otherwise
	// You will receive either:
	// ErrNotFound, ErrPasswordIncorrect, or another error if
	// something goes wrong.
	Authenticate(email, password string) (*User, error)
	UserDB
}

func NewUserService(db *mongo.Client, pepper, hmacKey string) UserService {
	ug := &userGorm{db}
	hmac := hash.NewHMAC(hmacKey)
	uv := newUserValidator(ug, hmac, pepper)
	return &userService{
		UserDB: uv,
		pepper: pepper,
	}
}

type userService struct {
	UserDB
	pepper string
}

func newUserValidator(udb UserDB, hmac hash.HMAC, pepper string) *userValidator {
	return &userValidator{
		UserDB: udb,
		hmac:   hmac,
		pepper: pepper,
		emailRegex: regexp.MustCompile(
			`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,16}$`),
	}
}

// Authenticate can be used to authenticate a user with the provided email address and password.
// If the email address provided is invalid, this will return nil, ErrNotFound
// If the password provided is invalid, this will return nil, ErrPasswordIncorrect
// If the email and password are both valid, this will return user, nil
// Otherwise if another error is encountered this will return nil, error
func (us *userService) Authenticate(email, password string) (*User, error) {

	foundUser, err := us.ByEmail(email)

	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(foundUser.PasswordHash), []byte(password+us.pepper))

	switch err {
	case nil:
		return foundUser, nil
	case bcrypt.ErrMismatchedHashAndPassword:
		return nil, ErrPasswordIncorrect
	default:
		return nil, err
	}
}

// userGorm represents our database interaction layer
// and implements the UserDB interface fully.
type userGorm struct {
	db *mongo.Client
}

// ByID will look up a user with the provided ID.
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with
// more information about what went wrong. This may not be
// an error generated by the models package.
//
// As a general rule, any error but ErrNotFound should
// probably result in a 500 error.
func (ug *userGorm) ByID(id string) (*User, error) {
	collection := ug.db.Database("lenslocked_dev").Collection("users")
	var user User
	filter := bson.D{{"_id", id}}
	err := collection.FindOne(context.TODO(), filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ByEmail looks up a user with the given email address and
// returns that user.
// If the user is found, we will return a nil error
// If the user is not found, we will return ErrNotFound
// If there is another error, we will return an error with
// more information about what went wrong. This may not be
// an error generated by the models package.
func (ug *userGorm) ByEmail(email string) (*User, error) {
	collection := ug.db.Database("lenslocked_dev").Collection("users")
	var user User
	filter := bson.D{{"email", email}}
	err := collection.FindOne(context.TODO(), filter).Decode(&user)
	return &user, err

}

// ByRemember looks up a user with the given remember token
// and returns that user. This method expects the remember
// token to already be hashed.
func (ug *userGorm) ByRemember(rememberHash string) (*User, error) {
	collection := ug.db.Database("lenslocked_dev").Collection("users")
	var user User
	filter := bson.D{{"remember_hash", rememberHash}}
	err := collection.FindOne(context.TODO(), filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Create will create the provided user and backfill data
// like the ID, CreatedAt, and UpdatedAt fields.
func (ug *userGorm) Create(user *User) error {
	collection := ug.db.Database("lenslocked_dev").Collection("users")
	_, err := collection.InsertOne(context.TODO(), user)
	return err
}

// Update will update the provided user with all of the data
// in the provided user object.
func (ug *userGorm) Update(user *User) error {
	collection := ug.db.Database("lenslocked_dev").Collection("users")
	filter := bson.D{{"_id", user.ID}}
	_, err := collection.UpdateOne(context.TODO(), filter, user)
	return err
}

// Delete will delete the user with the provided ID
func (ug *userGorm) Delete(id string) error {
	collection := ug.db.Database("lenslocked_dev").Collection("users")
	filter := bson.D{{"_id", id}}
	_, err := collection.DeleteOne(context.TODO(), filter)
	return err
}

// first will query using the provided gorm.DB and it will
// get the first item returned and place it into dst. If
// nothing is found in the query, it will return ErrNotFound
func first(db *gorm.DB, dst interface{}) error {
	err := db.First(dst).Error
	if err == gorm.ErrRecordNotFound {
		return ErrNotFound
	}
	return err
}

// userValidator is our validation layer that validates
// and normalizes data before passing it on to the next
// UserDB in our interface chain.
type userValidator struct {
	UserDB
	hmac       hash.HMAC
	emailRegex *regexp.Regexp
	pepper     string
}

// ByEmail will normalize an email address before passing
// it on to the database layer to perform the query.
func (uv *userValidator) ByEmail(email string) (*User, error) {
	user := User{
		Email: email,
	}
	err := runUserValFns(&user, uv.normalizeEmail)
	if err != nil {
		return nil, err
	}
	return uv.UserDB.ByEmail(user.Email)
}

func (uv *userValidator) ByRemember(token string) (*User, error) {
	user := User{
		Remember: token,
	}
	if err := runUserValFns(&user, uv.hmacRemember); err != nil {
		return nil, err
	}
	return uv.UserDB.ByRemember(user.RememberHash)
}

// Create will create the provided user and backfill data
// like the ID, CreatedAt, and UpdatedAt fields.
func (uv *userValidator) Create(user *User) error {
	err := runUserValFns(user,
		uv.passwordRequired,
		uv.passwordMinLength,
		uv.bcryptPassword,
		uv.passwordHashRequired,
		uv.setRememberIfUnset,
		uv.rememberMinBytes,
		uv.hmacRemember,
		uv.rememberHashRequired,
		uv.normalizeEmail,
		uv.requireEmail,
		uv.emailFormat,
		uv.emailIsAvail)
	if err != nil {
		return err
	}
	return uv.UserDB.Create(user)
}

// Update will hash a remember token if it is provided.
func (uv *userValidator) Update(user *User) error {
	err := runUserValFns(user,
		uv.passwordMinLength,
		uv.bcryptPassword,
		uv.passwordHashRequired,
		uv.rememberMinBytes,
		uv.hmacRemember,
		uv.rememberHashRequired,
		uv.normalizeEmail,
		uv.requireEmail,
		uv.emailFormat,
		uv.emailIsAvail)
	if err != nil {
		return err
	}
	return uv.UserDB.Update(user)
}

// Delete will delete the user with the provided ID
func (uv *userValidator) Delete(id string) error {

	/*
		var user User
		user.ID = id
		err := runUserValFns(&user, uv.idGreaterThan(0))
		if err != nil {
			return err
		}
	*/

	return uv.UserDB.Delete(id)
}

func runUserValFns(user *User, fns ...userValFn) error {
	for _, fn := range fns {
		if err := fn(user); err != nil {
			return err
		}
	}
	return nil
}

type userValFn func(*User) error

// bcryptPassword will hash a user's password with an
// app-wide pepper and bcrypt, which salts for us.
func (uv *userValidator) bcryptPassword(user *User) error {

	if user.Password == "" {
		// We DO NOT need to run this if the password
		// hasn't been changed.
		return nil
	}

	pwBytes := []byte(user.Password + uv.pepper)
	hashedBytes, err := bcrypt.GenerateFromPassword(pwBytes,
		bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hashedBytes)
	user.Password = ""
	return nil
}

func (uv *userValidator) hmacRemember(user *User) error {
	if user.Remember == "" {
		return nil
	}
	user.RememberHash = uv.hmac.Hash(user.Remember)
	return nil
}

func (uv *userValidator) setRememberIfUnset(user *User) error {
	if user.Remember != "" {
		return nil
	}
	token, err := rand.RememberToken()
	if err != nil {
		return err
	}
	user.Remember = token
	return nil
}

func (uv *userValidator) idGreaterThan(n string) userValFn {

	return userValFn(func(user *User) error {

		/*
			if user.ID <= n {
				return ErrIDInvalid
			}
		*/

		return nil
	})

}

func (uv *userValidator) normalizeEmail(user *User) error {
	user.Email = strings.ToLower(user.Email)
	user.Email = strings.TrimSpace(user.Email)
	return nil
}

func (uv *userValidator) requireEmail(user *User) error {
	if user.Email == "" {
		return ErrEmailRequired
	}
	return nil
}

func (uv *userValidator) emailFormat(user *User) error {
	if user.Email == "" {
		return nil
	}
	if !uv.emailRegex.MatchString(user.Email) {
		return ErrEmailInvalid
	}
	return nil
}

func (uv *userValidator) emailIsAvail(user *User) error {
	existing, err := uv.ByEmail(user.Email)
	if err == ErrNotFound {
		// Email address is available if we don't find
		// a user with that email address.
		return nil
	}
	// We can't continue our validation without a successful
	// query, so if we get any error other than ErrNotFound we
	// should return it.
	if err != nil {
		return err
	}

	// If we get here that means we found a user w/ this email
	// address, so we need to see if this is the same user we
	// are updating, or if we have a conflict.
	if user.ID != existing.ID {
		return ErrEmailTaken
	}
	return nil
}

func (uv *userValidator) passwordMinLength(user *User) error {
	if user.Password == "" {
		return nil
	}
	if len(user.Password) < 8 {
		return ErrPasswordTooShort
	}
	return nil
}

func (uv *userValidator) passwordRequired(user *User) error {
	if user.Password == "" {
		return ErrPasswordRequired
	}
	return nil
}

func (uv *userValidator) passwordHashRequired(user *User) error {
	if user.PasswordHash == "" {
		return ErrPasswordRequired
	}
	return nil
}

func (uv *userValidator) rememberMinBytes(user *User) error {
	if user.Remember == "" {
		return nil
	}
	n, err := rand.NBytes(user.Remember)
	if err != nil {
		return err
	}
	if n < 32 {
		return ErrRememberTooShort
	}
	return nil
}

func (uv *userValidator) rememberHashRequired(user *User) error {
	if user.RememberHash == "" {
		return ErrRememberRequired
	}
	return nil
}

type modelError string

func (e modelError) Error() string {
	return string(e)
}

func (e modelError) Public() string {
	s := strings.Replace(string(e), "models: ", "", 1)
	split := strings.Split(s, " ")
	split[0] = strings.Title(split[0])
	return strings.Join(split, " ")
}
