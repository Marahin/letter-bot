package eventhandler

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/test/factories"
	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func TestHandler_OnBookWhenSuccessfulWithNoConflicting(t *testing.T) {
	// given
	assert := assert.New(t)
	request := book.BookRequest{
		Guild:          factories.CreateGuild(),
		Member:         factories.CreateMember(),
		Spot:           strconv.FormatInt(factories.CreateSpot().ID, 10),
		StartAt:        time.Now(),
		EndAt:          time.Now().Add(2 * time.Hour),
		Overbook:       false,
		HasPermissions: false,
	}
	bookingOperations := new(mocks.MockBookingService)
	bookingOperations.On("Book", request).Return([]*reservation.ClippedOrRemovedReservation{}, nil)
	adapter := NewHandler(
		bookingOperations,
		mocks.NewMockReservationRepository(t),
		mocks.NewMockCommunicationService(t),
		mocks.NewMockSummaryService(t),
	)

	// when
	response, err := adapter.OnBook(request)

	// assert
	assert.Nil(err)
	assert.NotNil(response)
	assert.Empty(response.ConflictingReservations)
}

func TestHandler_OnBookWhenOnUnsuccessful(t *testing.T) {
	// given
	assert := assert.New(t)
	request := book.BookRequest{
		Guild:          factories.CreateGuild(),
		Member:         factories.CreateMember(),
		Spot:           strconv.FormatInt(factories.CreateSpot().ID, 10),
		StartAt:        time.Now(),
		EndAt:          time.Now().Add(2 * time.Hour),
		Overbook:       false,
		HasPermissions: false,
	}
	bookingOperations := new(mocks.MockBookingService)
	bookingOperations.On("Book", request).Return([]*reservation.ClippedOrRemovedReservation{}, errors.New("error"))
	adapter := NewHandler(
		bookingOperations,
		mocks.NewMockReservationRepository(t),
		mocks.NewMockCommunicationService(t),
		mocks.NewMockSummaryService(t),
	)

	// when
	response, err := adapter.OnBook(request)

	// assert
	assert.NotNil(err)
	assert.Equal("error", err.Error())
	assert.NotNil(response)
}

func TestHandler_OnBookAutocompleteOverbookField(t *testing.T) {
	// given
	assert := assert.New(t)
	adapter := NewHandler(
		new(mocks.MockBookingService),
		mocks.NewMockReservationRepository(t),
		mocks.NewMockCommunicationService(t),
		mocks.NewMockSummaryService(t),
	)
	request := book.BookAutocompleteRequest{
		Field: book.BookAutocompleteOverbook,
	}

	// when
	res, err := adapter.OnBookAutocomplete(request)

	// assert
	assert.Nil(err)
	assert.Exactly(book.BookAutocompleteResponse{"true", "false"}, res)
}

func TestHandler_OnBookAutocompleteStartAtField(t *testing.T) {
	// given
	assert := assert.New(t)
	bookingOperations := new(mocks.MockBookingService)
	bookingOperations.On("GetSuggestedHours", mock.MatchedBy(mocks.TimeMatchedCloseTo), "").Return([]string{"01:00", "02:00"})
	adapter := NewHandler(
		bookingOperations,
		mocks.NewMockReservationRepository(t),
		mocks.NewMockCommunicationService(t),
		mocks.NewMockSummaryService(t),
	)
	request := book.BookAutocompleteRequest{
		Field: book.BookAutocompleteStartAt,
		Value: "",
	}

	// when
	res, err := adapter.OnBookAutocomplete(request)

	// assert
	assert.Nil(err)
	assert.Exactly(book.BookAutocompleteResponse{"01:00", "02:00"}, res)
}

func TestHandler_OnBookAutocompleteEndAtField(t *testing.T) {
	// given
	assert := assert.New(t)
	bookingOperations := new(mocks.MockBookingService)
	bookingOperations.On("GetSuggestedHours", mock.MatchedBy(mocks.NewMatcherForTimeAndTolerance(
		time.Now().Add(2*time.Hour),
		2*time.Millisecond,
	)), "").Return([]string{"03:00", "04:00"})
	adapter := NewHandler(
		bookingOperations,
		mocks.NewMockReservationRepository(t),
		mocks.NewMockCommunicationService(t),
		mocks.NewMockSummaryService(t),
	)
	request := book.BookAutocompleteRequest{
		Field: book.BookAutocompleteEndAt,
		Value: "",
	}

	// when
	res, err := adapter.OnBookAutocomplete(request)

	// assert
	assert.Nil(err)
	assert.Exactly(book.BookAutocompleteResponse{"03:00", "04:00"}, res)
}

func TestHandler_OnBookAutocompleteSpotField(t *testing.T) {
	// given
	assert := assert.New(t)
	bookingOperations := new(mocks.MockBookingService)
	bookingOperations.On("FindAvailableSpots", "asdf").Return([]string{"spot1", "spot2"}, nil)
	adapter := NewHandler(
		bookingOperations,
		mocks.NewMockReservationRepository(t),
		mocks.NewMockCommunicationService(t),
		mocks.NewMockSummaryService(t),
	)
	request := book.BookAutocompleteRequest{
		Field: book.BookAutocompleteSpot,
		Value: "asdf",
	}

	// when
	res, err := adapter.OnBookAutocomplete(request)

	// assert
	assert.Nil(err)
	assert.Exactly(book.BookAutocompleteResponse{"spot1", "spot2"}, res)
}
