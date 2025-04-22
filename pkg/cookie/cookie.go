package cookie

import (
	"time"

	"github.com/valyala/fasthttp"
)

func Get(ctx *fasthttp.RequestCtx, name string) string {
	return string(ctx.Request.Header.Cookie(name))
}

func Set(ctx *fasthttp.RequestCtx, name string, value string, path, domain string, expiration time.Duration, secure bool, sameSite fasthttp.CookieSameSite) {
	cookie := fasthttp.AcquireCookie()

	cookie.SetKey(name)
	cookie.SetPath(path)
	cookie.SetHTTPOnly(true)
	// cookie.SetDomain(domain)
	cookie.SetValue(value)
	cookie.SetSameSite(sameSite)

	if expiration >= 0 {
		if expiration == 0 {
			cookie.SetExpire(fasthttp.CookieExpireUnlimited)
		} else {
			cookie.SetExpire(time.Now().Add(expiration))
		}
	}

	if secure {
		cookie.SetSecure(true)
	}

	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)

	fasthttp.ReleaseCookie(cookie)
}

func Delete(ctx *fasthttp.RequestCtx, name string) {
	ctx.Request.Header.DelCookie(name)
	ctx.Response.Header.DelCookie(name)

	cookie := fasthttp.AcquireCookie()
	cookie.SetKey(name)
	cookie.SetValue("")
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	//RFC says 1 second, but let's do it 1 minute to make sure is working...
	exp := time.Now().Add(-1 * time.Minute)
	cookie.SetExpire(exp)
	ctx.Response.Header.SetCookie(cookie)

	fasthttp.ReleaseCookie(cookie)
}
