# Package rusty

Package `rusty` provides a flexible and easy-to-use API for interacting with server-side REST-ful resources.
It acts as a wrapper over an HTTP client providing higher-level abstractions.

## Table of Contents

- [Creating an Endpoint](#creating-an-endpoint)
- [Handling the response](#handling-the-response)
- [The HTTP client](#the-http-client)
- [Specifying a body](#specifying-a-body)
- [Specifying query string parameters](#specifying-query-string-parameters)
- [Target ID: What is it and Why is it Used?](#target-id-what-is-it-and-why-is-it-used)
- [Other options](#other-options)

## Creating an Endpoint

`rusty` main abstraction is the `rusty.Endpoint` struct which represents a resource that you can interact with by
calling its methods (e.g. `Get`, `Post`, `Put`, `Delete`, `Patch`).

It is created by calling the `rusty.NewEndpoint` function which takes the following mandatory parameters:
- An HTTP client that will be used to make the HTTP call.
- The URL of the resource that can be fixed or contain placeholders.

```go
endpoint, err := rusty.NewEndpoint(httpClient, "https://api.user.com/users/{id}")
if err != nil {
    // handle error
}

response, err := endpoint.Get(ctx, rusty.WithParam("id", 100))
if err != nil {
    // handle error
}

var user User
err = json.Unmarshal(response.Body, &user)
```

> See the [HTTP client](#the-http-client) section for more details on the `httpClient` parameter.

You should always instantiate the endpoint when _bootstrapping_ your application and share it across the lifetime of
the application since it is immutable and safe to use concurrently by multiple goroutines.

You can also use the `rusty.URL` function to create the URL instead of passing a string directly. This is useful when
you need to create a URL from multiple parts. For example, when you have a base URL and a path that you want to
concatenate. `rusty.URL` will take care of the trailing slashes and the missing ones.

## Handling the response

A `rusty.Endpoint` method call succeeds if `err` is nil. In that case, the response is guaranteed to be non-nil and
contain the HTTP response body.
The response body is a `[]byte` and it is the caller's responsibility to unmarshal it into the appropriate structure.

On the other hand, if `err` is not nil it means that the operation failed.
The possible causes are:

- The underlying HTTP call failed at transport level (timeout or network communication error).
- The underlying HTTP call succeeded but returned with an invalid response.

The first case is not handled by `rusty` and it is up to the caller to handle it. This means that
the error returned by the `rusty.Endpoint` method call will be the one returned by the HTTP client.
The second case is handled by the `rusty.ErrorPolicy` function. By default, it returns an error if the response code
is not in the 2xx range. You can change this default behavior by specifying a custom `rusty.ErrorPolicy` function on
`rusty.NewEndpoint` by calling `WithErrorPolicy` option.

```go
func main() {
	endpoint, err := rusty.NewEndpoint(httpClient, "https://api.user.com/users/{id}")
	if err != nil {
		return err
	}

	response, err := endpoint.Get(ctx, rusty.WithParam("id", "1"))
	if err != nil {
		var rustyErr *rusty.Error
		if errors.As(err, &rustyErr) {
			if rustyErr.StatusCode == http.StatusNotFound {
				// handle not found and return
			}
		}

		return err
	}
}
```

## The HTTP client

When creating a new `rusty.Endpoint` you must provide an HTTP client that will be used to make the HTTP call.
That means that any transport related configurations like timeouts or retries must be specified at the HTTP client
level.
The client must implement the following interface:

```go
type Requester interface {
    Do(*http.Request) *http.Response
}
```

Although you can use any HTTP client that implements the `Requester` interface, we recommend using the
[httpclient](https://github.com/luizaranda/go-core/tree/master/pkg/transport/httpclient) package from this
same module.

Although you can use the same client for one or more endpoints, keep in mind that how you configure it will depend on
that particular endpoint requirement. For example, some of them may be safe to retry, some may not. Some would require a
short timeout, some may not need a timeout at all.

## Specifying a body

To send a body along the request, we provide a `rusty.WithBody` option function.

```go
func main() {
	endpoint, err := rusty.NewEndpoint(httpClient, "https://api.user.com/users")
	if err != nil {
		return err
	}

	type User struct {
		Name     string `json:"name"`
		Lastname string `json:"lastname"`
	}

	response, err := endpoint.Post(ctx,
		rusty.WithHeader("Content-Type", "application/json"),
		rusty.WithBody(User{
			Name:     "Rob",
			Lastname: "Pike"
		}))
}

```

`rusty.WithBody` will detect the type of the body as defined by the `Content-Type` header and will marshal it
accordingly.
It also supports a `[]byte` or `io.Reader` body type which will be sent as is without any marshaling.

## Specifying query string parameters

Query string parameters are supported can be specified using the `WithQuery` option function.
For example, a `/search` endpoint that might accept different filter values.

```go
func main() {
	search, err := rusty.NewEndpoint(httpClient, rusty.URL("https://api.user.com/users/search"))
	if err != nil {
		return err
	}

	query := url.Values{}
	query.Add("is_active", "true")

	response, err := search.Get(ctx, rusty.WithQuery(query))
}
```

## Target ID: What is it and Why is it Used?

The target ID is a string designed to add a dimension to the metrics sent to our
observability backends. For instance, when using the HTTP client provided by the `httpclient`
package, it is sent as a target_id tag for platforms like Datadog.
This tag helps identify the destination of the call and observe its behavior.

It's crucial to note that the value of the target ID should have the lowest possible cardinality.
Therefore, it is recommended to use a fixed value for all endpoints that call the same destination.

For example, if there's an endpoint making calls to https://api.user.com/users and another
to https://api.user.com/users/{id}, you can use the same target ID for both since they are
calling the same destination. If you prefer different target IDs for each endpoint, it's advised to
use a fixed value for each and avoid dynamic values like the user's id. For instance, you could use
`users` as the target ID for the first endpoint and `users/_id` for the second one.

Additionally, if defined, the target ID is used as the bucket key for
the [Circuit Breaker RoundTripper](/pkg/transport/README.md#circuit-breaker-roundtripper).

```go
endpoint, err := rusty.NewEndpoint(httpClient, "https://api.user.com/users/{id}",
	rusty.WithTargetID("users/_id"))
if err != nil {
    // handle error
}
```

## Other options

We recommend you to dive into the code to discover all the available options.
There are two types of options: the ones when creating the `rusty.Endpoint` and the ones that can be used when calling
the endpoint functions (like `Get` or `Post`).

There is a special treatment for the `WithHeader` option function. Any header specified at the request level will be
used in that particular request, and the ones specified at the endpoint level will be sent in every request. There is
one thing to note, though: if you specify the same header in the endpoint as in the request, the one set in the request
will take precedence.
