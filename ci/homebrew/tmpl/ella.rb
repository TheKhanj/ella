class Ella < Formula
  desc "A process manager for running and managing services"
  homepage "https://github.com/thekhanj/ella"
  version ""
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/thekhanj/ella/releases/download/v#{version}/ella_v#{version}_darwin_arm64.tar.gz"
    else
      url "https://github.com/thekhanj/ella/releases/download/v#{version}/ella_v#{version}_darwin_amd64.tar.gz"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/thekhanj/ella/releases/download/v#{version}/ella_v#{version}_linux_arm64.tar.gz"
    else
      url "https://github.com/thekhanj/ella/releases/download/v#{version}/ella_v#{version}_linux_amd64.tar.gz"
    end
  end

  def install
    bin.install "ella"
    doc.install "fsm/service.png"
    doc.install "fsm/service-deactivating.png"
    man1.install "man/ella.1.gz"
    bash_completion.install "completion.sh"
  end

  test do
    assert_match "ella version", shell_output("#{bin}/ella -version")
  end
end
