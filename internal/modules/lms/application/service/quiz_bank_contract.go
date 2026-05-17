package service

func quizPackageFromQuiz(quiz Quiz) QuizPackageResponseDTO {
	detail := quizDetailToDTO(quiz)
	hash := quiz.PackageHash
	if hash == "" {
		hash = hashAny(detail)
	}
	return QuizPackageResponseDTO{
		Version:     quiz.Version,
		PackageHash: hash,
		ContentHash: hash,
		GradingMode: "server",
		Quiz:        detail,
	}
}
