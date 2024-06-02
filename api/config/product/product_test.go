package product

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/config/storage"
)

func TestFetch(t *testing.T) {
	assert := assert.New(t)
	productConfig := New(mockStorageClient(), config.ClusterId{
		App:     "pay",
		Region:  "central",
		Variant: "default",
	})

	tarStream, err := productConfig.Fetch("0.0.21")

	assert.Greater(len(tarStream), 0)
	assert.Nil(err)
}

func mockStorageClient() *MockStorageClient {
	mockStorageClient := new(MockStorageClient)

	mockStorageObject := new(MockStorageObject)
	mockStorageObject.On("GetName").Return("config-app=pay,hash=xyz,version=0.0.21.tar.gz")
	mockStorageObject.On("GetContents").Return([]byte("test-content"), nil)
	mockStorageClient.On("ListObjects", "arryved-app-control-config").Return(
		[]storage.StorageObject{
			mockStorageObject,
		},
		nil,
	)

	return mockStorageClient
}

type MockStorageClient struct {
	mock.Mock
}

func (m *MockStorageClient) ListObjects(bucketName string) ([]storage.StorageObject, error) {
	args := m.Called(bucketName)
	return args.Get(0).([]storage.StorageObject), args.Error(1)
}

type MockStorageObject struct {
	mock.Mock
}

func (m *MockStorageObject) GetContents() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockStorageObject) GetName() string {
	args := m.Called()
	return args.String(0)
}
