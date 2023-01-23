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

func (u *AudonUser) GetIndicator(ctx context.Context, fnew []byte) ([]byte, error) {
	if u == nil {
		return nil, errors.New("nil user")
	}

	mtype := mimetype.Detect(fnew)
	if !mimetype.EqualsAny(mtype.String(), "image/png", "image/jpeg", "image/webp", "image/gif") {
		return nil, errors.New("file type not supported")
	}

	hash := sha256.Sum256(fnew)
	isAvatarNew := false

	var err error

	// Check if user's original avatar exists
	saved := u.GetOriginalAvatarPath(hash, mtype)
	if _, err := os.Stat(saved); err != nil {
		if err := os.MkdirAll(filepath.Dir(saved), 0775); err != nil {
			return nil, err
		}
		// Write user's avatar if the original version doesn't exist
		if err := os.WriteFile(saved, fnew, 0664); err != nil {
			return nil, err
		}
		if u.AvatarFile != "" {
			os.Remove(u.getAvatarImagePath(u.AvatarFile))
		}
		isAvatarNew = true
	}

	fname := filepath.Base(saved)
	coll := mainDB.Collection(COLLECTION_USER)
	if _, err = coll.UpdateOne(ctx,
		bson.D{{Key: "audon_id", Value: u.AudonID}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "avatar", Value: fname}}},
		}); err != nil {
		return nil, err
	}

	if !isAvatarNew {
		if data, err := os.ReadFile(u.GetOriginalAvatarPath(hash, mtype)); err == nil {
			return data, nil
		}
	}

	buf := bytes.NewBuffer(fnew)

	var newImg image.Image
	if mtype.Is("image/png") {
		newImg, err = png.Decode(buf)
	} else if mtype.Is("image/jpeg") {
		newImg, err = jpeg.Decode(buf)
	} else if mtype.Is("image/webp") {
		newImg, err = webp.Decode(buf)
	} else if mtype.Is("image/gif") {
		newImg, err = gif.Decode(buf)
	}
	if err != nil {
		return nil, err
	}
	return u.createGIF(newImg)
}

func (u *AudonUser) createGIF(avatar image.Image) ([]byte, error) {
	avatarPNG := image.NewRGBA(image.Rect(0, 0, 150, 150))
	draw.BiLinear.Scale(avatarPNG, avatarPNG.Rect, avatar, avatar.Bounds(), draw.Src, nil)

	baseFrame := image.NewRGBA(avatarPNG.Bounds())
	draw.Draw(baseFrame, baseFrame.Bounds(), image.Black, image.Point{}, draw.Src)
	draw.Copy(baseFrame, image.Point{}, avatarPNG, avatarPNG.Bounds(), draw.Over, nil)
	draw.Draw(baseFrame, baseFrame.Bounds(), mainConfig.LogoImageBack, image.Point{-35, -35}, draw.Over)

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
		draw.DrawMask(frame, frame.Bounds(), mainConfig.LogoImageFront, image.Point{-35, -35}, mask, image.Point{}, draw.Over)

		if err := anim.AddFrame(frame, 1000/count*i, webpConf); err != nil {
			return nil, err
		}
	}

	outBuf, _ := os.Create(u.GetWebPAvatarPath())
	defer outBuf.Close()
	anim.Encode(outBuf)

	imagick.Initialize()
	defer imagick.Terminate()

	if _, err := imagick.ConvertImageCommand([]string{"convert", u.GetWebPAvatarPath(), u.GetGIFAvatarPath()}); err != nil {
		return nil, err
	}

	return os.ReadFile(u.GetGIFAvatarPath())
}

func (u *AudonUser) GetOriginalAvatarPath(hash [sha256.Size]byte, mtype *mimetype.MIME) string {
	filename := fmt.Sprintf("%x%s", hash, mtype.Extension())
	return u.getAvatarImagePath(filename)
}

func (u *AudonUser) GetGIFAvatarPath() string {
	return u.getAvatarImagePath("indicator.gif")
}

func (u *AudonUser) GetWebPAvatarPath() string {
	return u.getAvatarImagePath("indicator.webp")
}

func (u *AudonUser) getAvatarImagePath(name string) string {
	if u == nil {
		return ""
	}

	return filepath.Join(mainConfig.StorageDir, u.AudonID, "avatar", name)
}
