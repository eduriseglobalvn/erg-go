package repository

import communitydto "erg.ninja/internal/modules/community/api/dto"

type AuthorDTO = communitydto.AuthorDTO
type CommentDTO = communitydto.CommentDTO
type ListPostsQuery = communitydto.ListPostsQuery
type MediaDTO = communitydto.MediaDTO
type PostDTO = communitydto.PostDTO
type ReactionSummaryDTO = communitydto.ReactionSummaryDTO
type TopicDTO = communitydto.TopicDTO

const (
	TargetTypePost    = communitydto.TargetTypePost
	TargetTypeComment = communitydto.TargetTypeComment
	TargetTypeTopic   = communitydto.TargetTypeTopic
	TargetTypeUser    = communitydto.TargetTypeUser

	ReactionLike  = communitydto.ReactionLike
	ReactionLove  = communitydto.ReactionLove
	ReactionCare  = communitydto.ReactionCare
	ReactionHaha  = communitydto.ReactionHaha
	ReactionWow   = communitydto.ReactionWow
	ReactionSad   = communitydto.ReactionSad
	ReactionAngry = communitydto.ReactionAngry
)
