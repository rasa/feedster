# feedster 
[![feedster logo](https://cdn.pixabay.com/photo/2017/05/18/15/05/chipmunk-2323827__180.jpg)](# "feedster logo")

[![Build Status](https://travis-ci.com/rasa/feedster.svg)](https://travis-ci.com/rasa/feedster "Build status")
[![codebeat.co](https://codebeat.co/badges/3dc8a008-a9e2-46c1-bca9-bc00f6b38a25)](https://codebeat.co/projects/github-com-rasa-feedster-master "Codebeat.co")
[![codecov.io](https://codecov.io/gh/rasa/feedster/branch/master/graph/badge.svg)](https://codecov.io/gh/rasa/feedster "codecov.io") 
[![Libraries.io](https://img.shields.io/librariesio/github/rasa/feedster.svg)](https://libraries.io/github/rasa/feedster)
[![GolangCI](https://golangci.com/badges/github.com/rasa/feedster.svg)](https://golangci.com/r/github.com/rasa/feedster "GolangCI")
[![Go Report Card](https://goreportcard.com/badge/github.com/rasa/feedster)](https://goreportcard.com/report/github.com/rasa/feedster "Go Report Card")
[![MIT License](https://img.shields.io/github/license/rasa/feedster.svg?style=flat-square)](LICENSE "MIT License")

Easily add metadata (id3v2) tags to MP3 files and generate a podcast RSS feed for the Google Play Music, iTunes, Spotify, Stitcher, TuneIn and other podcast directory services.

<!-- toc -->

## Creating Your Podcast Feed

1. Download feedster from the [releases](../../releases) page (or install via scoop)
1. Update [default.yaml](default.yaml) and fill in at least the [`base_url`][base_url] field with the web site location where you will host the files for this podcast
1. Update [default-podcast.yaml](default-podcast.yaml) and fill in at least the [title][title], [link][link], and [description][description] fields
1. Update [default-tracks.csv](default-tracks.csv) with your tag settings (you can use an .xlsx, or .txt file instead, if you want, by setting [`tracks_file`][tracks_file] to the filename
1. Optionally, copy a .jpg image into the current directory and rename it `default.jpg.` Apple requires the image to be between 1400x1400 pixels and 3000x3000 pixels
1. Run `feedster default.yaml`
1. If successful, feedster will generate a podcast RSS feed named `default/default.xml`, and copy the related .jpg and .mp3 files into the `default/` directory. It also added the metadata (id3v2) tags to the .mp3 files.
1. Upload the files feedster created in the `default/` directory to the directory on your web site that cooresponds to the URL you entered in the [`base_url`][base_url] field to in [default.yaml](default.yaml)

## Testing Your Podcast Feed

Assuming in [default.yaml](default.yaml) you set the [`base_url`][base_url] field to 
`https://example.com/my-new-podcast/`, and you left the 
[`output_file`][output_file] field blank, your podcast feed URL would then be
`https://example.com/my-new-podcast/default.xml` as `output_file` defaults to the name of the .yaml file, `deault.yaml` in this case.

If you set [`output_file`][output_file] to, say, `my-new-podcast.xml`, 
your podcast feed URL would be `https://example.com/my-new-podcast/my-new-podcast.xml`.

To test your feed, open any browser, and enter your podcast feed URL into the URL field, and press Enter. If you don't see any errors, proceed to validate your podcast feed, using the following instructions.

## Validating Your Podcast Feed

You can validate your podcast by submitting its URL to one or more of the following feed validation services:

* [Podcast Validator](https://podba.se/validate/) *([Apple reccomended](https://help.apple.com/itc/podcasts_connect/#/itcac471c970))*
* [Cast Feed Validator](https://castfeedvalidator.com/)
* [W3C Feed Validation Service](https://validator.w3.org/feed/)
* [Feed Validator](https://www.feedvalidator.org/)

If you podcast feed validated successfully, submit it to one or more of the podcast directory services listed below.

## Submitting Your Podcast Feed

Submit your podcast feed URL to one or more of the following podcast directory services:

| Service | Account Required? | Submit Podcast |
|---------|-------------------|----------------|
| [Apple Podcasts (formally iTunes)](https://www.apple.com/itunes/podcasts/) | [Yes][apple-signup] | [Link][apple-submit] ([*instructions*](#apple-podcasts-)) |
| [Google Play Music](https://play.google.com/music/listen?u=0#/home) | [Yes][google-signup] | [Link][google-submit] ([*instructions*](#google-play-music-)) |
| [Spotify](https://open.spotify.com/genre/podcasts-page) | [Yes][spotify-signup] | [*Instructions*](#spotify-) |
| [Stitcher](https://www.stitcher.com/) | Yes | *[Instructions](#stitcher-)* |
| [TuneIn](https://tunein.com/) | No | *[Instructions](#tunein-)* |
| | | |
| [Acast](https://www.acast.com/) | No | *[Instructions](#acast-)* |
| [Blubrry](https://www.blubrry.com/) | Yes | *[Instructions](#blubrry-)* |
| [Digital Podcast](#) | | *[Instructions](#digital-podcast-)* |
| [doubleTwist](https://www.doubletwist.com/) | No | *[Instructions](#doubletwist-)* |
| [iHeartRadio](#) | | *[Instructions](#iheartradio-)* |
| [iPodder](#) | | *[Instructions](#ipodder-)* |
| [Libsyn](https://libsyn.com/) | Yes ($5/mo up) | *[Instructions](#libsyn-)* |
| [Player.fm](https://player.fm/) | No | *[Instructions](#playerfm-)* |
| [Podbean](https://www.podbean.com/) | Yes | *[Instructions](#podbean-)* | 
| [Podcast Blaster](https://www.podcastblaster.com/) | No | *[Instructions](#podcast-blaster-)* |
| [Podcastpedia](#) | | *[Instructions](#podcastpedia-)* |
| [Spreaker](#) | No | *[Instructions](#spreaker-)* |

Once you've received an email from Apple that your podcast has been published on iTunes, submit your **iTunes URL** (details [*here*](determining-your-itunes-url)) to any of the following services:

| Service | Account Required? | Submission Instructions/Notes |
|---------|-------------|--------------------|
| [Listen Notes](https://www.listennotes.com/submit/) | No | *[Instructions](#listen-notes-)* |
| [RadioPublic](https://radiopublic.typeform.com/) | No | *[Instructions](#radiopublic-)* |

### Refreshing Your Feed

* To refresh the feed on iTunes, visit https://podcastsconnect.apple.com/my-podcasts, 
select the podcast you want to refresh, and click "Refresh Feed".

## Podcast Submission Instructions

### Apple Podcasts [&#x2934;](#submitting-your-podcast-feed)

1. Sign in to Apple [here][apple-signin] using your iTunes login credentials (or sign-up using the [iTunes application][apple-signup] on your PC, iPad, or iPhone)
1. Visit Apple's [Podcasts Connect][apple-submit] to start the submission process
1. @TODO Verify the following:
1. Click the plus sign at the top of the dashboard
1. Enter your podcast feed URL, and click "Validate"
1. If your feed was accepted a feed preview will appear
1. Click "Submit"
1. You'll receive an email after your podcast is reviewed, which can take from 6 to 48 hours or more

Another step-by-step guide is available on [Podcast Insights](https://www.podcastinsights.com/upload-a-podcast/#Submit_Your_RSS_Feed_To_iTunes)

### Google Play Music [&#x2934;](#submitting-your-podcast-feed)

1. Sign in to Google [here][google-signin] (or sign up [here][google-signup])
1. Visit Google Play Music's [podcast submission page][google-submit]
1. Accept the terms and conditions (if prompted)
1. Enter your podcast feed URL into the field shown
1. Click "Submit RSS Feed"
1. @TODO verify the following:
1. Click "ADD A PODCAST"
1. Enter the information requested
1. Click "Submit"
1. You'll receive an email after your podcast is reviewed

Another step-by-step guide is available on [Podcast Insights](https://www.podcastinsights.com/submit-podcast-to-google-play/)

### Spotify [&#x2934;](#submitting-your-podcast-feed)

1. [Sign up][spotify-signup]

Another step-by-step guide is available on [Podcast Insights](https://www.podcastinsights.com/submit-podcast-to-spotify/)

### Stitcher [&#x2934;](#submitting-your-podcast-feed)

1. Visit Stitcher's [Sign up page]{stitcher-signup]
1.
1. Your podcast should appear on the site in [about an hour](https://stitcher.helpshift.com/a/stitcher-partners/?s=stitcher-partners-new-partner-faq&f=why-isn-t-my-show-up-yet&l=en).

### TuneIn [&#x2934;](#submitting-your-podcast-feed)

1. For support questions, email Content@Stitcher.com
1. Visit TuneIn's [podcast submission page][tunein-submit]
1. Enter the requested information
1. Accept the terms and conditions
1. Click "Send "
1. 

### Acast [&#x2934;](#submitting-your-podcast-feed)

1. Visit Acast's [podcast submission page](https://www.acast.com/podcasters)
1. Scroll to the bottom of the page
1. Click "Add Your Show"
1. Choose your adventure (non-hosted, hosted, hosted with a brand new show)
1. Enter the requested information
1. Click "Send"
1. Scroll UP on the resulting page to see the confirmation message
1.

### Blubrry [&#x2934;](#submitting-your-podcast-feed)

1. Sign in to Blubrry [here](#) (or sign up [here](https://www.blubrry.com/createaccount.php))
1. Visit Blubrry's [podcast submission page](#)
1. Enter your podcast feed URL
1. Enter a "web friendly" name for your podcast
1. Choose a category
1. Accept the terms and conditions
1. Click "Submit"
1. You'll receive an email after your podcast is reviewed

### Digital Podcast [&#x2934;](#submitting-your-podcast-feed)

1. http://www.digitalpodcast.com/feeds/new

### doubleTwist [&#x2934;](#submitting-your-podcast-feed)

1. Visit doubleTwist's [podcast submission page](#)
1. Enter your name
1. Enter your email address
1. For the `Subject` field, select "Request New Podcast"
1. Enter the title of your podcast
1. Enter the CAPTCHA information
1. Click "Submit"
1. 

### iHeartRadio [&#x2934;](#submitting-your-podcast-feed)

1. 

### iPodder [&#x2934;](#submitting-your-podcast-feed)

1. http://www.ipodder.org/hints/new

### Libsyn [&#x2934;](#submitting-your-podcast-feed)

1. https://signup.libsyn.com/

### Listen Notes [&#x2934;](#submitting-your-podcast-feed)

1. Visit Listen Notes' [podcast submission page](https://www.listennotes.com/submit/)
1. Enter the **iTunes URL** (details [*here*](determining-your-itunes-url)) of your podcast
1. Enter your email address *(to be emailed when your podcast's been added)*
1. Click "Submit"
1. 

### Player.fm [&#x2934;](#submitting-your-podcast-feed)

1. Visit Player.fm's [podcast submission page](https://player.fm/importer/new/)
1. Enter your podcast feed URL
1. Click "Import"
1. 

### Podbean [&#x2934;](#submitting-your-podcast-feed)

1. Sign in to Podbean [here](#) (or sign up [here](#))
1. Visit Podbean's [podcast submission page](https://www.podbean.com/site/submitPodcast)
1. Enter a username
1. Enter your podcast feed URL *(remove the https:// prefix as Podbean adds it by default)*
1. Click ...
1. Your podcast is now live

### Podcast Blaster [&#x2934;](#submitting-your-podcast-feed)

1. Visit Podcast Blaster's [podcast submission page](https://www.podcastblaster.com/directory/add-podcast/)
1. Enter your Podcast feed URL into the "Add podcast feed to directory" field
1. Click "Add Podcast"
1. 

### Podcastpedia [&#x2934;](#submitting-your-podcast-feed) 

1. https://www.podcastpedia.org/how_can_i_help/add_podcast

### RadioPublic [&#x2934;](#submitting-your-podcast-feed)

1. Visit RadioPublic's [podcast submission page](https://radiopublic.typeform.com/to/tWMwSl)
1. Enter the name of your podcast
1. Enter your email address
1. Enter your podcast feed URL (be sure to copy over the default http:// stuff if you need to)
1. Then the form asks for the link to your podcast in iTunes (now Apple Podcasts) – that could be a problem for you if you don’t know how to find it. So here you go…
1. 

### Spreaker [&#x2934;](#submitting-your-podcast-feed)

1. https://www.spreaker.com/signup
1. 

## Determining Your iTunes URL

## Contributing

Please read [CONTRIBUTING.md](https://gist.github.com/PurpleBooth/b24679402957c63ec426) for details on our code of conduct, and the process for submitting pull requests to us.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](tags). 

## Authors

* **Ross Smith II** - *Initial work* - [@rasa](../../..)

See also the list of [contributors](../../graphs/contributors) who participated in this project.

## License

This project is MIT Licensed - see [LICENSE.md](LICENSE.md) for details.

## Links

### Podcast Links

* https://help.apple.com/itc/podcasts_connect/#/itc2b3780e76
* https://www.podcastinsights.com/podcast-distribution-guide/
* https://podcastfasttrack.com/the-2017-2018-list-of-podcast-directories-your-podcast-must-be-listed-in-ep-79/

### RSS Links

* https://cyber.harvard.edu/rss/rss.html#optionalChannelElements
* https://github.com/simplepie/simplepie-ng/wiki/Spec:-iTunes-Podcast-RSS

### id3v2 Links

* https://help.mp3tag.de/main_tags.html
* http://id3.org/d3v2.3.0

[base_url]: default.yaml#L6
[output_file]: default.yaml#L62
[title]: default-podcast.yaml#L5
[link]: default-podcast.yaml#L7
[description]: default-podcast.yaml#L10
[tracks_file]: default.yaml#8
[apple-signin]: https://itunesconnect.apple.com/login?module=PodcastsConnect&hostname=podcastsconnect.apple.com&targetUrl=%2F&authResult=FAILED
[apple-signup]: https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/accountSummary
[apple-submit]: https://podcastsconnect.apple.com/
[google-signin]: https://accounts.google.com/signin/v2/identifier?service=sj&passive=1209600&continue=https%3A%2F%2Fplay.google.com%2Fmusic%2Fpodcasts%2Fpublish&followup=https%3A%2F%2Fplay.google.com%2Fmusic%2Fpodcasts%2Fpublish&flowName=GlifWebSignIn&flowEntry=ServiceLogin
[google-signup]: https://accounts.google.com/signup/v2/webcreateaccount?service=sj&continue=https%3A%2F%2Fplay.google.com%2Fmusic%2Fpodcasts%2Fpublish&flowName=GlifWebSignIn&flowEntry=SignUp
[google-submit]: https://g.co/podcastportal
[spotify-signup]: https://www.spotify.com/us/signup/
[stitcher-signup]: https://www.stitcher.com/content-providers
[tunein-submit]: https://help.tunein.com/contact/add-podcast-S19TR3Sdf
