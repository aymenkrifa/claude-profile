# claudeprofile.aymenkrifa.com

A Worker that serves one thing: the installer, at
`claudeprofile.aymenkrifa.com/install.sh`.

It proxies `install.sh` from `main` rather than holding a copy, so the script
people pipe into a shell is the same one they can read in the repository, and
changing it needs no deploy here. `/` redirects to the repository; anything
else is a 404 that names the installer's URL.

## Deploying

The zone `aymenkrifa.com` belongs to the **aymenkrifa@gmail.com** Cloudflare
account. This machine's default wrangler login is the *other* account
(acubic.admin@gmail.com), which cannot see the zone, so the two are kept apart
the same way the cloudflared certs are -- one config directory per account:

```sh
export XDG_CONFIG_HOME=~/.config/cloudflare-aymenkrifa
npx wrangler@4 login          # once, in a browser, as aymenkrifa@gmail.com
npx wrangler@4 deploy         # from this directory
```

Without the `XDG_CONFIG_HOME` override, `wrangler login` overwrites the
existing acubic.admin token in `~/.config/.wrangler`.

## Checking it

```sh
curl -sI https://claudeprofile.aymenkrifa.com/install.sh   # 200, text/plain
curl -fsSL https://claudeprofile.aymenkrifa.com/install.sh | sh -s -- --help
npx wrangler@4 tail                                        # live requests
```
