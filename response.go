package grequests

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"os"
)

type Response struct {

	// Ok is a boolean flag that validates that the server returned a 2xx code
	Ok bool

	// This is the Go error flag – if something went wrong within the request, this flag will be set.
	Error error

	// We want to abstract (at least at the moment) the Go http.Response object away. So we are going to make use of it
	// internal but not give the user access
	RawResponse *http.Response

	// StatusCode is the HTTP Status Code returned by the HTTP Response. Taken from resp.StatusCode
	StatusCode int

	// Header is a net/http/Header structure
	Header http.Header

	internalByteBuffer *bytes.Buffer
}

func buildResponse(resp *http.Response, err error) *Response {
	// If the connection didn't succeed we just return a blank response
	if err != nil {
		return &Response{Error: err}
	}

	return &Response{
		// If your code is within the 2xx range – the response is considered `Ok`
		Ok:          resp.StatusCode <= 200 && resp.StatusCode < 300,
		Error:       nil,
		RawResponse: resp,
		StatusCode:  resp.StatusCode,
		Header:      resp.Header,
	}
}

// Read is part of our ability to support io.ReadCloser if someone wants to make use of the raw body
func (r *Response) Read(p []byte) (n int, err error) {
	return r.RawResponse.Body.Read(p)
}

// Close is part of our ability to support io.ReadCloser if someone wants to make use of the raw body
func (r *Response) Close() error {
	return r.RawResponse.Body.Close()
}

// DownloadToFile allows you to download the contents of the response to a file
func (r *Response) DownloadToFile(fileName string) error {
	fd, err := os.Create(fileName)

	if err != nil {
		return err
	}

	defer r.Close() // This is a noop if we use the internal ByteBuffer
	defer fd.Close()

	if _, err := io.Copy(fd, r.getInternalReader()); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// getInternalReader because we implement io.ReadCloser and optionally hold a large buffer of the response (created by
// the user's request)
func (r *Response) getInternalReader() io.Reader {
	if r.internalByteBuffer != nil {
		return r.internalByteBuffer
	}
	return r
}

// Xml is a method that will populate a struct that is provided `userStruct` with the XML returned within the
// response body
func (r *Response) Xml(userStruct interface{}, charsetReader XMLCharDecoder) error {
	xmlDecoder := xml.NewDecoder(r.getInternalReader())

	if charsetReader != nil {
		xmlDecoder.CharsetReader = charsetReader
	}

	defer r.Close()

	if err := xmlDecoder.Decode(&userStruct); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// Json is a method that will populate a struct that is provided `userStruct` with the JSON returned within the
// response body
func (r *Response) Json(userStruct interface{}) error {
	jsonDecoder := json.NewDecoder(r.getInternalReader())
	defer r.Close()

	if err := jsonDecoder.Decode(&userStruct); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// respBytesBuffer is a utility method that will populate the internal byte reader – this is largely used for .String()
// and .Bytes()
func (r *Response) respBytesBuffer() error {

	if r.internalByteBuffer != nil {
		return nil
	}

	defer r.Close()

	r.internalByteBuffer = &bytes.Buffer{}
	r.internalByteBuffer.Grow(int(r.RawResponse.ContentLength))

	if _, err := io.Copy(r.internalByteBuffer, r); err != nil && err != io.EOF {
		return err
	}

	return nil

}

func (r *Response) Bytes() []byte {
	if err := r.respBytesBuffer(); err != nil {
		return nil
	}

	return r.internalByteBuffer.Bytes()

}

func (r *Response) String() string {
	if err := r.respBytesBuffer(); err != nil {
		return ""
	}

	return r.internalByteBuffer.String()
}

// ClearInternalBuffer is a function that will clear the internal buffer that we use to hold the .String() and .Bytes()
// data. Once you have used these functions – you may want to free up the memory.
func (r *Response) ClearInternalBuffer() {
	r.internalByteBuffer.Reset()
	r.internalByteBuffer = nil
}