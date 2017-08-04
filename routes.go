package main

import (
	"github.com/gin-gonic/gin"
	csh_auth "github.com/liam-middlebrook/csh-auth"
	log "github.com/sirupsen/logrus"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"
)

func protectedProfile(c *gin.Context) {
	claims, ok := c.Value(csh_auth.AuthKey).(csh_auth.CSHClaims)
	if !ok {
		log.Fatal("error finding claims")
		return
	}
	c.String(http.StatusOK, "uid %s email %s name %s uuid %s", claims.UserInfo.Username, claims.UserInfo.Email, claims.UserInfo.FullName, claims.UserInfo.Subject)
}

func index(c *gin.Context) {
	c.Redirect(http.StatusFound, "/upload")
}

func action(c *gin.Context) {
	plug := GetPlug()
	url := S3PresignPlug(plug)

	claims, ok := c.Value(csh_auth.AuthKey).(csh_auth.CSHClaims)
	if !ok {
		log.Fatal("error finding claims")
		return
	}
	log.WithFields(log.Fields{
		"uid":           claims.UserInfo.Username,
		"plug_id":       plug.ID,
		"plug_s3id":     plug.S3ID,
		"presigned_uri": url.String(),
	}).Info("Presigned URI Generated")
	c.Redirect(http.StatusFound, url.String())
}

func upload(c *gin.Context) {
	plug := Plug{}

	claims, ok := c.Value(csh_auth.AuthKey).(csh_auth.CSHClaims)
	if !ok {
		log.Fatal("error finding claims")
		return
	}

	plug.Owner = claims.UserInfo.Username
	plug.ViewsRemaining = 100

	if !DecrementCredits(plug.Owner, 1) {
		c.String(http.StatusPaymentRequired, "Get More Credits!")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		log.Error(err)
	}
	log.Info(file.Filename)
	data, err := file.Open()
	if err != nil {
		log.Error(err)
	}
	defer data.Close()
	imageData, _, err := image.DecodeConfig(data)
	if err != nil {
		log.Error(err)
	}
	data.Seek(0, 0)
	if imageData.Width == 728 && imageData.Height == 200 {
		mime := getMime(data)
		data.Seek(0, 0)

		plug.S3ID = time.Now().Format("2006/01/02/150405") + "-" + plug.Owner + "-" + file.Filename
		S3AddFile(plug, data, mime)

		MakePlug(plug)
	} else {
		log.Error("invalid file dimensions")
	}

	c.Data(http.StatusOK, "text/html", []byte(`
	<html>
	<body>
		<h2>Uploaded a Plug!</h2>
		<p>Take a look at what you uploaded! (This does not count towards the views for your Plug!)</p>
		<div>
			<img src="`+S3PresignPlug(plug).String()+`"></img>
		</div>
	</body>
	</html>
	`))
	log.WithFields(log.Fields{
		"uid":       claims.UserInfo.Username,
		"plug_id":   plug.ID,
		"plug_s3id": plug.S3ID,
	}).Info("Uploaded new Plug!")
}

func upload_view(c *gin.Context) {
	c.Data(http.StatusOK, "text/html", []byte(`
	<html>
	<body>
		<h2>Upload a Plug!</h2>
		<p>You will lose 1 drink credit in exchange for a 100 view-limit plug!</p>
		<div>
			<form action="/upload" method="post" enctype="multipart/form-data">
				<input type="file" name="file" id="file">
				<input type="submit" value="Upload" name="submit">
			</form>
		</div>
	</body>
	</html>
	`))
}

func getMime(data io.Reader) string {
	buffer := make([]byte, 512)
	n, err := data.Read(buffer)
	if err != nil {
		log.Error(err)
	}
	return http.DetectContentType(buffer[:n])
}
