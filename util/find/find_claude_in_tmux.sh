#!/usr/bin/env bash

echo "분석 결과: 'claude' 프로세스가 세션에서 완전히 분리됨을 확인했습니다."
echo "프로세스 상속 관계를 재귀적으로 추적하여 'claude' 프로세스를 찾습니다."
echo "-------------------------------------"

# 시스템의 모든 프로세스 정보를 미리 한 번만 읽어 성능을 최적화합니다. (PID와 부모 PID)
# macOS 환경을 고려하여 ps -ax -o pid,ppid를 사용합니다.
all_processes=$(ps -ax -o pid,ppid)

# 특정 PID의 모든 자손 프로세스 PID를 찾아 'descendants' 배열에 추가하는 재귀 함수
find_all_descendants() {
    local parent_pid=$1
    
    # 미리 읽어둔 정보에서 부모 PID가 일치하는 자식 프로세스들을 찾습니다.
    local children=$(echo "$all_processes" | awk -v ppid="$parent_pid" '$2 == ppid {print $1}')
    
    for child_pid in $children; do
        # 찾은 자식 PID를 결과 배열에 추가합니다.
        descendants+=("$child_pid")
        # 해당 자식의 자손을 찾기 위해 재귀적으로 함수를 다시 호출합니다.
        find_all_descendants "$child_pid"
    done
}

# 현재 실행 중인 모든 'claude' 프로세스의 PID 목록을 가져옵니다.
claude_pids=($(ps -eo pid,comm | awk '$2 == "claude" {print $1}'))

if [ ${#claude_pids[@]} -eq 0 ]; then
    echo "실행 중인 'claude' 프로세스를 찾을 수 없습니다."
    exit 0
fi

echo "'claude' 프로세스 ${#claude_pids[@]}개를 찾았습니다. tmux pane과 연결을 시도합니다."
echo "-------------------------------------"

# 모든 tmux pane을 순회합니다.
tmux list-panes -a -F '#{pane_id} #{pane_pid}' | while read -r pane_id pane_pid; do
    
    # 현재 pane의 자손 목록을 초기화합니다.
    descendants=()
    
    # 현재 pane의 첫 프로세스(pane_pid)의 모든 자손을 찾습니다.
    find_all_descendants "$pane_pid"
    
    # 찾은 자손 목록에 'claude' 프로세스의 PID가 있는지 확인합니다.
    for c_pid in "${claude_pids[@]}"; do
        # 배열에 특정 PID가 포함되어 있는지 검사
        if [[ " ${descendants[*]} " =~ " ${c_pid} " ]]; then
            echo "tmux pane '$pane_id'에서 claude 프로세스를 찾았습니다!"
            echo "  - Pane ID: $pane_id"
            echo "  - Pane의 셸 PID: $pane_pid"
            echo "  - Claude 프로세스 ID (PID): $c_pid"
            echo "-------------------------------------"
        fi
    done
done
