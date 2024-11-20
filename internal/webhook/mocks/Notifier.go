// Code generated by mockery v2.48.0. DO NOT EDIT.

package mocks

import (
	github "github.com/google/go-github/v66/github"
	mock "github.com/stretchr/testify/mock"
)

// Notifier is an autogenerated mock type for the Notifier type
type Notifier struct {
	mock.Mock
}

type Notifier_Expecter struct {
	mock *mock.Mock
}

func (_m *Notifier) EXPECT() *Notifier_Expecter {
	return &Notifier_Expecter{mock: &_m.Mock}
}

// Notify provides a mock function with given fields: repository, gazers
func (_m *Notifier) Notify(repository *github.Repository, gazers []*github.Stargazer) {
	_m.Called(repository, gazers)
}

// Notifier_Notify_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Notify'
type Notifier_Notify_Call struct {
	*mock.Call
}

// Notify is a helper method to define mock.On call
//   - repository *github.Repository
//   - gazers []*github.Stargazer
func (_e *Notifier_Expecter) Notify(repository interface{}, gazers interface{}) *Notifier_Notify_Call {
	return &Notifier_Notify_Call{Call: _e.mock.On("Notify", repository, gazers)}
}

func (_c *Notifier_Notify_Call) Run(run func(repository *github.Repository, gazers []*github.Stargazer)) *Notifier_Notify_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*github.Repository), args[1].([]*github.Stargazer))
	})
	return _c
}

func (_c *Notifier_Notify_Call) Return() *Notifier_Notify_Call {
	_c.Call.Return()
	return _c
}

func (_c *Notifier_Notify_Call) RunAndReturn(run func(*github.Repository, []*github.Stargazer)) *Notifier_Notify_Call {
	_c.Call.Return(run)
	return _c
}

// NewNotifier creates a new instance of Notifier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewNotifier(t interface {
	mock.TestingT
	Cleanup(func())
}) *Notifier {
	mock := &Notifier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
