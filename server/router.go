package server

import (
	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/server/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Init(e *gin.Engine, database *db.Database) {
	e.GET("/health", handles.Health)
	e.GET("/help", handles.Help)
	e.GET("/posts", handles.GetPosts(database))
	e.GET("/post/:pid", handles.GetPost(database))
	e.GET("/comment", handles.GetComment(database))
	e.GET("/comments/:pid", handles.GetComments(database))
	e.GET("/media/image", handles.GetImage)

	v3 := e.Group("/chapi/api/v3")
	{
		v3.GET("/hole/list_comments", handles.V3ListComments(database))
		v3.GET("/hole/one", handles.V3HoleOne(database))
		v3.GET("/hole/get", handles.V3HoleGet(database))
		v3.GET("/comment/list", handles.V3CommentList(database))
		v3.GET("/message/un_read", handles.V3MessageUnread)
		v3.GET("/message/index", handles.V3MessageIndex)
		v3.GET("/tags/tree", handles.V3TagsTree)
		v3.GET("/media/getThumbnail", handles.GetImage)
		v3.GET("/media/getMediaBinary", handles.GetImage)
		v3.GET("/media/getImageBinary", handles.GetImage)
	}

	Cors(e)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}
