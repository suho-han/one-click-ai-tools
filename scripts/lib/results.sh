SUCCESS_ITEMS=()
FAILED_ITEMS=()

record_success() {
    SUCCESS_ITEMS+=("$1")
}

record_failure() {
    FAILED_ITEMS+=("$1")
    echo -e "${RED}Failed: $1${NC}"
}

summarize_results() {
    echo -e "${BLUE}=== Summary ===${NC}"
    echo -e "${GREEN}Succeeded: ${#SUCCESS_ITEMS[@]}${NC}"
    for item in "${SUCCESS_ITEMS[@]}"; do
        echo -e "  ${GREEN}- ${item}${NC}"
    done

    if [[ ${#FAILED_ITEMS[@]} -gt 0 ]]; then
        echo -e "${RED}Failed: ${#FAILED_ITEMS[@]}${NC}"
        for item in "${FAILED_ITEMS[@]}"; do
            echo -e "  ${RED}- ${item}${NC}"
        done
    else
        echo -e "${GREEN}Failed: 0${NC}"
    fi
}
