package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sizeofint/webpanimation"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
	"gopkg.in/gographics/imagick.v2/imagick"
)

func (u *AudonUser) GetIndicator(ctx context.Context, fnew []byte, room *Room) (indicator []byte, original []byte, isGIF bool, err error) {
	isGIF = false

	if u == nil {
		err = errors.New("nil user")
		return
	}

	mtype := mimetype.Detect(fnew)
	if !mimetype.EqualsAny(mtype.String(), "image/png", "image/jpeg", "image/webp", "image/gif") {
		err = errors.New("file type not supported")
		return
	}

	buf := bytes.NewReader(fnew)

	var newImg image.Image
	if mtype.Is("image/png") {
		newImg, err = png.Decode(buf)
	} else if mtype.Is("image/jpeg") {
		newImg, err = jpeg.Decode(buf)
	} else if mtype.Is("image/webp") {
		newImg, err = webp.Decode(buf)
	} else if mtype.Is("image/gif") {
		newImg, err = gif.Decode(buf)
		isGIF = true
	}
	if err != nil {
		return
	}

	// encode to png to avoid recompression, except GIF
	var origImg []byte
	if !isGIF {
		origBuf := new(bytes.Buffer)
		if err = png.Encode(origBuf, newImg); err != nil {
			return
		}
		origImg = origBuf.Bytes()
	} else {
		origImg = fnew
	}
	hash := sha256.Sum256(origImg)

	// Check if user's original avatar exists
	var filename string
	if isGIF {
		filename = fmt.Sprintf("%x.gif", hash)
	} else {
		filename = fmt.Sprintf("%x.png", hash)
	}
	saved := u.getAvatarImagePath(filename)
	if _, err = os.Stat(saved); err != nil {
		if err = os.MkdirAll(filepath.Dir(saved), 0775); err != nil {
			return
		}
		// Write user's avatar if the original version doesn't exist
		if err = os.WriteFile(saved, origImg, 0664); err != nil {
			return
		}
	}

	coll := mainDB.Collection(COLLECTION_USER)
	if _, err = coll.UpdateOne(ctx,
		bson.D{{Key: "audon_id", Value: u.AudonID}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "avatar", Value: filename}}},
		}); err != nil {
		return
	}

	indicator, err = u.createGIF(newImg, room.IsHost(u) || room.IsCoHost(u))
	if err != nil {
		return
	}

	return indicator, origImg, isGIF, nil
}

func (u *AudonUser) createGIF(avatar image.Image, blue bool) ([]byte, error) {
	avatarPNG := image.NewRGBA(image.Rect(0, 0, 150, 150))
	draw.BiLinear.Scale(avatarPNG, avatarPNG.Rect, avatar, avatar.Bounds(), draw.Src, nil)

	baseFrame := image.NewRGBA(avatarPNG.Bounds())
	draw.Draw(baseFrame, baseFrame.Bounds(), image.Black, image.Point{}, draw.Src)
	draw.Copy(baseFrame, image.Point{}, avatarPNG, avatarPNG.Bounds(), draw.Over, nil)
	logoImageBack := mainConfig.LogoImageWhiteBack
	if blue {
		logoImageBack = mainConfig.LogoImageBlueBack
	}
	draw.Draw(baseFrame, baseFrame.Bounds(), logoImageBack, image.Point{-55, -105}, draw.Over)

	anim := webpanimation.NewWebpAnimation(150, 150, 0)
	defer anim.ReleaseMemory()
	webpConf := webpanimation.NewWebpConfig()
	webpConf.SetLossless(1)

	count := 20

	for i := 0; i < count; i++ {
		frame := image.NewRGBA(baseFrame.Bounds())
		draw.Copy(frame, image.Point{}, baseFrame, baseFrame.Bounds(), draw.Src, nil)

		var alpha uint8
		if i < count/2 {
			alpha = uint8(255. * (1. - float32(2*i)/float32(count)))
		} else {
			alpha = uint8(255. * (float32(2*i)/float32(count) - 1.))
		}

		mask := image.NewUniform(color.Alpha{alpha})
		draw.DrawMask(frame, frame.Bounds(), mainConfig.LogoImageFront, image.Point{-55, -105}, mask, image.Point{}, draw.Over)

		if err := anim.AddFrame(frame, 1000/count*i, webpConf); err != nil {
			return nil, err
		}
	}

	outBuf, _ := os.Create(u.getWebPAvatarPath())
	defer outBuf.Close()
	anim.Encode(outBuf)

	imagick.Initialize()
	defer imagick.Terminate()

	if _, err := imagick.ConvertImageCommand([]string{"convert", u.getWebPAvatarPath(), u.getGIFAvatarPath()}); err != nil {
		return nil, err
	}

	return os.ReadFile(u.getGIFAvatarPath())
}

func (u *AudonUser) getGIFAvatarPath() string {
	return u.getAvatarImagePath("indicator.gif")
}

func (u *AudonUser) getWebPAvatarPath() string {
	return u.getAvatarImagePath("indicator.webp")
}

func (u *AudonUser) getAvatarImagePath(name string) string {
	if u == nil {
		return ""
	}

	return filepath.Join(mainConfig.StorageDir, u.AudonID, "avatar", name)
}
